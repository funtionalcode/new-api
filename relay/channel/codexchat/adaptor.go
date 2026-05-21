package codexchat

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
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
	chatGPTReq := convertToChatGPTConversation(request)

	// 注入系统提示
	if info.ChannelSetting.SystemPrompt != "" {
		systemMsg := chatGPTMessage{
			ID:      "system-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Author:  messageAuthor{Role: "system"},
			Content: messageContent{ContentType: "text", Parts: []string{info.ChannelSetting.SystemPrompt}},
			Metadata: map[string]any{
				"is_visually_hidden_from_conversation": true,
			},
		}
		chatGPTReq.Messages = append([]chatGPTMessage{systemMsg}, chatGPTReq.Messages...)
	}

	return chatGPTReq, nil
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

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	key := strings.TrimSpace(info.ApiKey)

	// 缓冲请求体用于重试
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read request body failed: %w", err)
	}

	proxy := info.ChannelSetting.Proxy
	cookie := ensureCfClearance(c, info.ChannelId, info.ChannelBaseUrl, key, proxy)

	// 首次请求
	resp, cfErr := a.doCodexChatRequest(c, info, bodyBytes, key, cookie)
	if cfErr != nil {
		return nil, cfErr
	}

	// 检测 Cloudflare 挑战
	if isCloudflareChallenge(resp) {
		logger.LogWarn(c, fmt.Sprintf("codexchat: Cloudflare challenge detected for channel %d, retrying with fresh cookie", info.ChannelId))
		invalidateCfClearance(info.ChannelId)
		resp.Body.Close()

		// 重新获取 cookie 并重试
		cookie = ensureCfClearance(c, info.ChannelId, info.ChannelBaseUrl, key, proxy)
		resp, cfErr = a.doCodexChatRequest(c, info, bodyBytes, key, cookie)
		if cfErr != nil {
			return nil, cfErr
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

// doCodexChatRequest 构建并发送请求到 chatgpt.com/backend-api/conversation
func (a *Adaptor) doCodexChatRequest(c *gin.Context, info *relaycommon.RelayInfo, bodyBytes []byte, apiKey string, cookie string) (*http.Response, error) {
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
	req.Header.Set("Origin", info.ChannelBaseUrl)
	req.Header.Set("Referer", info.ChannelBaseUrl+"/")
	req.Header.Set("originator", "codex_cli_rs")

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

	// 注入 Cloudflare cookie
	injectCfClearance(req, cookie)

	// 发送请求
	resp, err := channel.DoRequest(c, req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.IsStream {
		return a.handleStreamResponse(c, resp, info)
	}
	return a.handleNonStreamResponse(c, resp, info)
}

// handleStreamResponse 处理流式响应
func (a *Adaptor) handleStreamResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)

	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	model := info.UpstreamModelName

	events := parseChatGPTSSEStream(resp.Body)
	defer resp.Body.Close()

	responseText := ""
	for event := range events {
		if event.Type == "message_stream_complete" {
			break
		}

		chunk := convertToOpenAIStreamChunk(event, model, respID)
		if chunk == nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
			responseText += *chunk.Choices[0].Delta.Content
		}

		if err := helper.ObjectData(c, chunk); err != nil {
			logger.LogError(c, "codexchat: write SSE chunk failed: "+err.Error())
			break
		}
	}

	_ = helper.ObjectData(c, buildStopChunk(model, respID))
	helper.Done(c)

	// 构建 usage
	promptTokens := info.GetEstimatePromptTokens()
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: countTokens(responseText),
		TotalTokens:      promptTokens + countTokens(responseText),
	}
	return usage, nil
}

// handleNonStreamResponse 处理非流式响应
func (a *Adaptor) handleNonStreamResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	defer resp.Body.Close()

	events := parseChatGPTSSEStream(resp.Body)

	responseText := ""
	for event := range events {
		if event.Type == "message_stream_complete" {
			break
		}
		if event.Message == nil {
			continue
		}
		if event.Message.Content.ContentType == "text" && len(event.Message.Content.Parts) > 0 {
			responseText += strings.Join(event.Message.Content.Parts, "")
		}
	}

	promptTokens := info.GetEstimatePromptTokens()
	completionTokens := countTokens(responseText)
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}

	// 返回 OpenAI 格式的响应
	response := dto.OpenAITextResponse{
		Id:      fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   info.UpstreamModelName,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: responseText,
				},
				FinishReason: "stop",
			},
		},
		Usage: *usage,
	}

	c.JSON(http.StatusOK, response)
	return usage, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/backend-api/conversation", info.ChannelType), nil
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

// countTokens 简单估算 token 数（约 4 字符 = 1 token）
func countTokens(text string) int {
	if text == "" {
		return 0
	}
	// 简单估算：中文约 1.5 字符/token，英文约 4 字符/token
	runes := []rune(text)
	estimated := len(runes) / 3
	if estimated < 1 {
		estimated = 1
	}
	return estimated
}
