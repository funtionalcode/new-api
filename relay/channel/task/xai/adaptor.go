package xai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

type videoRequest struct {
	Model           string         `json:"model,omitempty"`
	Prompt          string         `json:"prompt"`
	Duration        int            `json:"duration,omitempty"`
	AspectRatio     string         `json:"aspect_ratio,omitempty"`
	Resolution      string         `json:"resolution,omitempty"`
	Image           *videoImage    `json:"image,omitempty"`
	ReferenceImages []videoImage   `json:"reference_images,omitempty"`
	Output          map[string]any `json:"output,omitempty"`
	StorageOptions  map[string]any `json:"storage_options,omitempty"`
	User            string         `json:"user,omitempty"`
}

type videoImage struct {
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
}

type submitResponse struct {
	RequestID string `json:"request_id"`
}

type videoResultResponse struct {
	Status   string `json:"status"`
	Model    string `json:"model"`
	Progress int    `json:"progress"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Video *struct {
		URL      string `json:"url"`
		Duration int    `json:"duration"`
	} `json:"video"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	a.apiKey = info.ApiKey
	if info.ChannelMeta != nil {
		a.baseURL = info.ChannelMeta.ChannelBaseUrl
	}
	if a.baseURL == "" {
		a.baseURL = info.ChannelBaseUrl
	}
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, "generate")
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return strings.TrimRight(a.baseURL, "/") + "/v1/videos/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	modelName := req.Model
	if info != nil && info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	body := videoRequest{
		Model:    modelName,
		Prompt:   req.Prompt,
		Duration: req.Duration,
	}
	if body.Duration == 0 && strings.TrimSpace(req.Seconds) != "" {
		if parsedSeconds := intFromString(req.Seconds); parsedSeconds > 0 {
			body.Duration = parsedSeconds
		}
	}
	if imageURL := firstNonEmpty(req.Image, firstString(req.Images)); imageURL != "" {
		body.Image = &videoImage{URL: imageURL}
	}
	if req.Size != "" {
		if strings.Contains(req.Size, ":") {
			body.AspectRatio = req.Size
		} else {
			body.Resolution = req.Size
		}
	}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &body); err != nil {
		return nil, err
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var submit submitResponse
	if err := common.Unmarshal(responseBody, &submit); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
	}
	if strings.TrimSpace(submit.RequestID) == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("missing request_id"), "invalid_response", http.StatusInternalServerError)
	}

	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.CreatedAt = time.Now().Unix()
	video.Model = info.OriginModelName
	c.JSON(http.StatusOK, video)
	return submit.RequestID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var result videoResultResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	taskInfo := &relaycommon.TaskInfo{}
	if result.Progress > 0 {
		taskInfo.Progress = fmt.Sprintf("%d%%", result.Progress)
	}
	status := strings.ToLower(strings.TrimSpace(result.Status))
	switch status {
	case "done", "completed", "succeeded", "success":
		taskInfo.Status = string(model.TaskStatusSuccess)
		taskInfo.Progress = "100%"
		if result.Video != nil {
			taskInfo.Url = result.Video.URL
		}
	case "failed", "error", "cancelled", "canceled":
		taskInfo.Status = string(model.TaskStatusFailure)
		taskInfo.Progress = "100%"
		if result.Error != nil {
			taskInfo.Reason = firstNonEmpty(result.Error.Message, result.Error.Code)
		}
	case "queued", "pending":
		taskInfo.Status = string(model.TaskStatusQueued)
	case "running", "processing", "in_progress":
		taskInfo.Status = string(model.TaskStatusInProgress)
	default:
		if result.Error != nil {
			taskInfo.Status = string(model.TaskStatusFailure)
			taskInfo.Reason = firstNonEmpty(result.Error.Message, result.Error.Code)
			taskInfo.Progress = "100%"
			return taskInfo, nil
		}
		return nil, fmt.Errorf("unknown xai video status: %s", result.Status)
	}
	if taskInfo.Progress == "0%" && taskInfo.Status == string(model.TaskStatusInProgress) {
		taskInfo.Progress = taskcommon.ProgressInProgress
	}
	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := task.ToOpenAIVideo()
	if task.FinishTime > 0 {
		video.CompletedAt = task.FinishTime
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{"grok-imagine-video", "grok-imagine-video-1.5"}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "xai"
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intFromString(value string) int {
	var out int
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &out)
	return out
}
