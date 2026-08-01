package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
	if constant.StreamingTimeout == 0 {
		constant.StreamingTimeout = 30
	}
}

func TestOpenaiSTTHandlerUsesDurationAsAudioTokens(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"text":"hello","duration":3.45}`)),
	}
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(10)

	newAPIError, usage := OpenaiSTTHandler(c, resp, info, "json")

	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 67, usage.PromptTokens)
	assert.Equal(t, 67, usage.TotalTokens)
	assert.Equal(t, 67, usage.PromptTokensDetails.AudioTokens)
}

func TestOpenaiSTTHandlerNormalizesUsageDetails(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"text":"hello","usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15}}`)),
	}

	newAPIError, usage := OpenaiSTTHandler(c, resp, &relaycommon.RelayInfo{}, "json")

	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 12, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)
	assert.Equal(t, 12, usage.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 3, usage.CompletionTokenDetails.TextTokens)
}

func TestOpenaiSTTHandlerAcceptsSingularInputTokenDetails(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"text":"hello","usage":{"input_tokens":12,"output_tokens":3,"total_tokens":15,"input_token_details":{"text_tokens":0,"audio_tokens":12}}}`,
		)),
	}

	newAPIError, usage := OpenaiSTTHandler(c, resp, &relaycommon.RelayInfo{}, "json")

	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 12, usage.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 0, usage.PromptTokensDetails.TextTokens)
	assert.Equal(t, 3, usage.CompletionTokenDetails.TextTokens)
}

func TestOpenaiSTTHandlerStreamUsesFinalUsage(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"transcript.text.delta\",\"delta\":\"hel\"}\n\n" +
				"data: {\"type\":\"transcript.text.done\",\"text\":\"hello\",\"usage\":{\"input_tokens\":8,\"output_tokens\":2,\"total_tokens\":10,\"input_token_details\":{\"audio_tokens\":8}}}\n\n",
		)),
	}
	info := &relaycommon.RelayInfo{IsStream: true}
	info.SetEstimatePromptTokens(99)

	newAPIError, usage := OpenaiSTTHandler(c, resp, info, "json")

	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 8, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)
	assert.Equal(t, 10, usage.TotalTokens)
	assert.Equal(t, 8, usage.PromptTokensDetails.AudioTokens)
	assert.Contains(t, w.Body.String(), "transcript.text.delta")
	assert.Contains(t, w.Body.String(), "transcript.text.done")
}
