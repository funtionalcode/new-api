package codexchat

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("codexchat channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("codexchat channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("codexchat channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("codexchat channel: endpoint not supported")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	// 直接透传，不转换格式
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("codexchat channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("codexchat channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("codexchat channel: /v1/responses endpoint not supported")
}

const codexChatUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.7103.113 Safari/537.36"

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	key := strings.TrimSpace(info.ApiKey)

	// 缓冲请求体用于重试
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}

	// 首次请求
	resp, err := a.doCodexChatRequest(c, info, bodyBytes, key)
	if err != nil {
		return nil, err
	}

	// 检测 Cloudflare 挑战（安全网，正常情况下不应触发）
	if isCloudflareChallenge(resp) {
		logger.LogWarn(c, fmt.Sprintf("codexchat: Cloudflare challenge detected for channel %d, retrying", info.ChannelId))
		resp.Body.Close()
		resp, err = a.doCodexChatRequest(c, info, bodyBytes, key)
		if err != nil {
			return nil, err
		}
		if isCloudflareChallenge(resp) {
			resp.Body.Close()
			return nil, types.NewError(
				errors.New("codexchat upstream returned a Cloudflare challenge page after retry"),
				types.ErrorCodeDoRequestFailed,
			)
		}
	}

	return resp, nil
}

// doCodexChatRequest 构建并发送请求
func (a *Adaptor) doCodexChatRequest(c *gin.Context, info *relaycommon.RelayInfo, bodyBytes []byte, apiKey string) (*http.Response, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	logger.LogDebug(c, "codexchat fullRequestURL: %s", fullRequestURL)

	req, err := http.NewRequest(c.Request.Method, fullRequestURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}

	// 设置基础 headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", codexChatUserAgent)

	// 应用 header override
	headerOverride, err := channel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for k, value := range headerOverride {
		req.Header.Set(k, value)
		if strings.EqualFold(k, "Host") {
			req.Host = value
		}
	}

	// 发送请求
	resp, err := channel.DoRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// 使用标准 OpenAI 响应处理
	if info.IsStream {
		usage, err = openai.OaiStreamHandler(c, info, resp)
	} else {
		usage, err = openai.OpenaiHandler(c, info, resp)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/chat/completions", info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	if key == "" {
		return errors.New("codexchat channel: key is required")
	}

	req.Set("Authorization", "Bearer "+key)
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	}

	return nil
}
