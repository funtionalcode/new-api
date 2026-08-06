package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedCountTokensRequest struct {
	path          string
	body          []byte
	authorization string
	xAPIKey       string
	version       string
	beta          string
}

func TestRelayClaudeCountTokensPassesThroughWithoutBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	captured := make(chan capturedCountTokensRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		captured <- capturedCountTokensRequest{
			path:          r.URL.RequestURI(),
			body:          body,
			authorization: r.Header.Get("Authorization"),
			xAPIKey:       r.Header.Get("x-api-key"),
			version:       r.Header.Get("anthropic-version"),
			beta:          r.Header.Get("anthropic-beta"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream-Marker", "count-tokens")
		_, _ = w.Write([]byte(`{"input_tokens":37}`))
	}))
	defer upstream.Close()

	rawRequest := `{"model":"claude-client-model","messages":[{"role":"user","content":"hello"}],"vendor_extension":{"keep":true}}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens?beta=true", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("anthropic-version", "2023-06-01")
	ctx.Request.Header.Set("anthropic-beta", "context-1m-2025-08-07")
	setCountTokensRelayContext(ctx, constant.ChannelTypeNewAPI, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelModelMapping, `{"claude-client-model":"claude-upstream-model"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 0)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)

	Relay(ctx, types.RelayFormatClaude)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"input_tokens":37}`, recorder.Body.String())
	assert.Equal(t, "count-tokens", recorder.Header().Get("X-Upstream-Marker"))

	var request capturedCountTokensRequest
	select {
	case request = <-captured:
	default:
		require.FailNow(t, "upstream did not receive the count_tokens request")
	}
	assert.Equal(t, "/v1/messages/count_tokens?beta=true", request.path)
	assert.Equal(t, "Bearer downstream-key", request.authorization)
	assert.Equal(t, "downstream-key", request.xAPIKey)
	assert.Equal(t, "2023-06-01", request.version)
	assert.Equal(t, "context-1m-2025-08-07", request.beta)

	var forwarded map[string]any
	require.NoError(t, common.Unmarshal(request.body, &forwarded))
	assert.Equal(t, "claude-upstream-model", forwarded["model"])
	assert.Equal(t, map[string]any{"keep": true}, forwarded["vendor_extension"])
	_, hasMaxTokens := forwarded["max_tokens"]
	assert.False(t, hasMaxTokens)
}

func TestRelayClaudeCountTokensUsesLocalEstimatorWhenChannelHasNoNativeEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() {
		constant.CountToken = oldCountToken
	})

	rawRequest := `{"model":"claude-client-model","messages":[{"role":"user","content":"hello from Claude Desktop"}]}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	setCountTokensRelayContext(ctx, constant.ChannelTypeOpenAI, "")
	common.SetContextKey(ctx, constant.ContextKeyChannelModelMapping, `{"claude-client-model":"grok-count-model"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 0)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)

	Relay(ctx, types.RelayFormatClaude)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		InputTokens int `json:"input_tokens"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Positive(t, response.InputTokens)
}

func TestRelayClaudeCountTokensUsesAnthropicEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	capturedPath := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath <- r.URL.RequestURI()
		if r.Header.Get("x-api-key") != "downstream-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"input_tokens":11}`))
	}))
	defer upstream.Close()

	rawRequest := `{"model":"claude-client-model","messages":[{"role":"user","content":"hello"}]}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens?beta=true", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	setCountTokensRelayContext(ctx, constant.ChannelTypeAnthropic, upstream.URL)

	Relay(ctx, types.RelayFormatClaude)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"input_tokens":11}`, recorder.Body.String())
	select {
	case path := <-capturedPath:
		assert.Equal(t, "/v1/messages/count_tokens?beta=true", path)
	default:
		require.FailNow(t, "Anthropic upstream did not receive the count_tokens request")
	}
}

func TestRelayClaudeDesktopMessagesTokenCountProbeStaysLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() {
		constant.CountToken = oldCountToken
	})

	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case upstreamCalled <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	rawRequest := `{
		"model":"claude-client-model",
		"messages":[{"role":"user","content":"count"}],
		"max_tokens":1,
		"tools":[{
			"name":"workspace_lookup",
			"description":"Look up detailed workspace information without invoking the model",
			"input_schema":{"type":"object","properties":{"query":{"type":"string"}}}
		}]
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	setCountTokensRelayContext(ctx, constant.ChannelTypeOpenAI, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelModelMapping, `{"claude-client-model":"grok-count-model"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 0)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)

	Relay(ctx, types.RelayFormatClaude)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Type  string `json:"type"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "message", response.Type)
	assert.Equal(t, "claude-client-model", response.Model)
	assert.Positive(t, response.Usage.InputTokens)
	assert.Zero(t, response.Usage.OutputTokens)
	select {
	case <-upstreamCalled:
		require.FailNow(t, "legacy token-count probe was forwarded upstream")
	default:
	}
}

func TestRelayClaudeDesktopChatCompletionsTokenCountProbeStaysLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() {
		constant.CountToken = oldCountToken
	})

	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case upstreamCalled <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	rawRequest := `{
		"model":"claude-client-model",
		"messages":[{"role":"user","content":"count"}],
		"max_tokens":1,
		"tools":[{
			"type":"function",
			"function":{
				"name":"workspace_lookup",
				"description":"Look up detailed workspace information without invoking the model",
				"parameters":{"type":"object","properties":{"query":{"type":"string"}}}
			}
		}]
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	setCountTokensRelayContext(ctx, constant.ChannelTypeOpenAI, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelModelMapping, `{"claude-client-model":"grok-count-model"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 0)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)

	Relay(ctx, types.RelayFormatOpenAI)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Object string `json:"object"`
		Model  string `json:"model"`
		Usage  struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, "claude-client-model", response.Model)
	assert.Positive(t, response.Usage.PromptTokens)
	assert.Zero(t, response.Usage.CompletionTokens)
	assert.Equal(t, response.Usage.PromptTokens, response.Usage.TotalTokens)
	select {
	case <-upstreamCalled:
		require.FailNow(t, "converted token-count probe was forwarded upstream")
	default:
	}
}

func TestRelayClaudeDesktopResponsesTokenCountProbeStaysLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() {
		constant.CountToken = oldCountToken
	})

	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case upstreamCalled <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	rawRequest := `{
		"model":"claude-client-model",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"count"}]}],
		"max_output_tokens":1,
		"tools":[{
			"type":"function",
			"name":"workspace_lookup",
			"description":"Look up detailed workspace information without invoking the model",
			"parameters":{"type":"object","properties":{"query":{"type":"string"}}}
		}]
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(rawRequest))
	ctx.Request.Header.Set("Content-Type", "application/json")
	setCountTokensRelayContext(ctx, constant.ChannelTypeOpenAI, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelModelMapping, `{"claude-client-model":"grok-count-model"}`)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, 0)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)

	Relay(ctx, types.RelayFormatOpenAIResponses)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Object string `json:"object"`
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "response", response.Object)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, "claude-client-model", response.Model)
	require.Len(t, response.Output, 1)
	assert.Equal(t, "message", response.Output[0].Type)
	assert.Equal(t, "completed", response.Output[0].Status)
	assert.Equal(t, "assistant", response.Output[0].Role)
	require.Len(t, response.Output[0].Content, 1)
	assert.Equal(t, "output_text", response.Output[0].Content[0].Type)
	assert.Empty(t, response.Output[0].Content[0].Text)
	assert.Positive(t, response.Usage.InputTokens)
	assert.Zero(t, response.Usage.OutputTokens)
	assert.Equal(t, response.Usage.InputTokens, response.Usage.TotalTokens)
	select {
	case <-upstreamCalled:
		require.FailNow(t, "Responses token-count probe was forwarded upstream")
	default:
	}
}

func setCountTokensRelayContext(c *gin.Context, channelType int, baseURL string) {
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-client-model")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelName, "count-tokens-test")
	common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, baseURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "downstream-key")
}
