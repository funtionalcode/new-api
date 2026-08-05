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

func setCountTokensRelayContext(c *gin.Context, channelType int, baseURL string) {
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-client-model")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyChannelName, "count-tokens-test")
	common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, baseURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "downstream-key")
}
