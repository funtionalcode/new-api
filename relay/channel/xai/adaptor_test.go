package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLForcesImageGenerationEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://example.com",
			ChannelType:    constant.ChannelTypeXai,
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1/images/generations", requestURL)
}

func TestGetRequestURLForcesSTTEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeAudioTranscription,
		RequestURLPath: "/v1/audio/transcriptions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://example.com",
			ChannelType:    constant.ChannelTypeXai,
		},
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1/stt", requestURL)
}

func TestConvertAudioRequestOmitsRoutingModelForXAI(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-stt"))
	require.NoError(t, writer.WriteField("language", "en"))
	require.NoError(t, writer.WriteField("url", "https://example.com/audio.mp3"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "grok-stt"})

	require.NoError(t, err)
	converted, err := io.ReadAll(reader)
	require.NoError(t, err)
	convertedBody := string(converted)
	assert.NotContains(t, convertedBody, `name="model"`)
	assert.Contains(t, convertedBody, `name="language"`)
	assert.Contains(t, convertedBody, `name="url"`)
	assert.True(t, strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data; boundary="))
}

func TestConvertAudioRequestWritesFileLastForXAI(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, err := writer.CreateFormFile("file", "audio.mp3")
	require.NoError(t, err)
	_, err = filePart.Write([]byte("audio"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("language", "en"))
	require.NoError(t, writer.WriteField("format", "true"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	reader, err := (&Adaptor{}).ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "grok-stt"})

	require.NoError(t, err)
	converted, err := io.ReadAll(reader)
	require.NoError(t, err)
	convertedBody := string(converted)
	languageIndex := strings.Index(convertedBody, `name="language"`)
	formatIndex := strings.Index(convertedBody, `name="format"`)
	fileIndex := strings.Index(convertedBody, `name="file"`)
	require.NotEqual(t, -1, languageIndex)
	require.NotEqual(t, -1, formatIndex)
	require.NotEqual(t, -1, fileIndex)
	assert.Greater(t, fileIndex, languageIndex)
	assert.Greater(t, fileIndex, formatIndex)
}

func TestXAIStreamHandlerPreservesUsageDetails(t *testing.T) {
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	chunk := `{"id":"chatcmpl-xai","object":"chat.completion.chunk","created":1770000000,"model":"grok-4.5","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}],"usage":{"prompt_tokens":1200,"completion_tokens":0,"total_tokens":1234,"prompt_tokens_details":{"cached_tokens":700,"cache_write_tokens":300,"text_tokens":500},"completion_tokens_details":{"reasoning_tokens":11}}}`
	body := "data: " + chunk + "\n\n" + "data: [DONE]\n\n"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-4.5",
		},
	}

	usage, err := xAIStreamHandler(c, info, resp)

	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 1200, usage.PromptTokens)
	assert.Equal(t, 34, usage.CompletionTokens)
	assert.Equal(t, 1234, usage.TotalTokens)
	assert.Equal(t, 700, usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 300, usage.PromptTokensDetails.CacheWriteTokens)
	assert.Equal(t, 500, usage.PromptTokensDetails.TextTokens)
	assert.Equal(t, 11, usage.CompletionTokenDetails.ReasoningTokens)
	assert.Equal(t, 23, usage.CompletionTokenDetails.TextTokens)
	assert.Contains(t, recorder.Body.String(), `"cached_tokens":700`)
}

func TestNormalizeXAIUsageMapsInputDetailsToPromptDetails(t *testing.T) {
	usage := &dto.Usage{
		InputTokens:  100,
		OutputTokens: 8,
		InputTokensDetails: &dto.InputTokenDetails{
			CachedTokens:     80,
			CacheWriteTokens: 20,
			TextTokens:       100,
		},
	}

	normalizeXAIUsage(usage)

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 8, usage.CompletionTokens)
	assert.Equal(t, 108, usage.TotalTokens)
	assert.Equal(t, 80, usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 20, usage.PromptTokensDetails.CacheWriteTokens)
	assert.Equal(t, 100, usage.PromptTokensDetails.TextTokens)
	assert.Equal(t, 8, usage.CompletionTokenDetails.TextTokens)
}
