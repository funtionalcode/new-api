package xai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyUsesXAIImageObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:    "grok-imagine-video-1.5",
		Prompt:   "make the water crash down",
		Image:    "https://example.com/frame.png",
		Duration: 12,
	})
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video-1.5"},
	}

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)

	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"grok-imagine-video-1.5",
		"prompt":"make the water crash down",
		"image":{"url":"https://example.com/frame.png"},
		"duration":12
	}`, string(bodyBytes))
}

func TestDoResponseReturnsOpenAIVideoWithPublicTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"xai_request"}`)),
	}

	taskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	require.Equal(t, "xai_request", taskID)
	require.JSONEq(t, `{"request_id":"xai_request"}`, string(taskData))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"task_public"`)
	require.Contains(t, recorder.Body.String(), `"model":"grok-imagine-video"`)
}

func TestParseTaskResultMapsDoneResponse(t *testing.T) {
	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"status":"done",
		"progress":100,
		"video":{"url":"https://example.com/video.mp4"},
		"usage":{"cost_in_usd_ticks":500000000}
	}`))

	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), taskInfo.Status)
	require.Equal(t, "100%", taskInfo.Progress)
	require.Equal(t, "https://example.com/video.mp4", taskInfo.Url)
}

func TestConvertToOpenAIVideoIncludesStoredURL(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_public",
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		CreatedAt: 1783566400,
		UpdatedAt: 1783566460,
		Properties: model.Properties{
			OriginModelName: "grok-imagine-video",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/video.mp4",
		},
	}

	data, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)

	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(data, &video))
	require.Equal(t, "task_public", video.ID)
	require.Equal(t, dto.VideoStatusCompleted, video.Status)
	require.Equal(t, "https://example.com/video.mp4", video.Metadata["url"])
}
