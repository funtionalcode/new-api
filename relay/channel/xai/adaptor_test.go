package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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

func TestDoRequestUsesWebsocketForResponses(t *testing.T) {
	type requestSnapshot struct {
		path          string
		authorization string
		contentType   string
	}

	observed := make(chan requestSnapshot, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		observed <- requestSnapshot{
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
		}
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsWebsocket:    true,
		RelayMode:      relayconstant.RelayModeResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: server.URL,
			ChannelType:    constant.ChannelTypeXai,
			ApiType:        constant.APITypeXai,
			ApiKey:         "upstream-secret",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	result, err := adaptor.DoRequest(c, info, nil)

	require.NoError(t, err)
	conn, ok := result.(*websocket.Conn)
	require.True(t, ok)
	require.NoError(t, conn.Close())

	request := <-observed
	assert.Equal(t, "/v1/responses", request.path)
	assert.Equal(t, "Bearer upstream-secret", request.authorization)
	assert.Equal(t, "application/json", request.contentType)
}

func TestConvertOpenAIResponsesRequestNormalizesRootUnionSchema(t *testing.T) {
	request := dto.OpenAIResponsesRequest{
		Model: "grok-4.5",
		Tools: []byte(`[
			{"type":"namespace","name":"mcp__codex_app","tools":[
				{"type":"function","name":"automation_update","strict":false,"parameters":{
					"type":"object","properties":{},
					"oneOf":[{"$ref":"#/$defs/create"},{"$ref":"#/$defs/delete"}],
					"$defs":{
						"id":{"type":"string"},
						"create":{"type":"object","properties":{"mode":{"enum":["create"]}},"required":["mode"],"additionalProperties":false},
						"delete":{"type":"object","properties":{"mode":{"enum":["delete"]},"id":{"$ref":"#/$defs/id"}},"required":["mode","id"],"additionalProperties":false}
					}
				}}
			]},
			{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}
		]`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	var tools []any
	require.NoError(t, common.Unmarshal(convertedRequest.Tools, &tools))
	require.Len(t, tools, 2)
	namespace, ok := tools[0].(map[string]any)
	require.True(t, ok)
	nestedTools, ok := namespace["tools"].([]any)
	require.True(t, ok)
	require.Len(t, nestedTools, 1)
	automationTool, ok := nestedTools[0].(map[string]any)
	require.True(t, ok)
	parameters, ok := automationTool["parameters"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, parameters, 1)
	assert.Contains(t, parameters, "oneOf")
	variants, ok := parameters["oneOf"].([]any)
	require.True(t, ok)
	require.Len(t, variants, 2)
	deleteVariant, ok := variants[1].(map[string]any)
	require.True(t, ok)
	properties, ok := deleteVariant["properties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "string"}, properties["id"])
	encodedTools := string(convertedRequest.Tools)
	assert.NotContains(t, encodedTools, `"$defs"`)
	assert.NotContains(t, encodedTools, `"$ref"`)
	assert.Contains(t, encodedTools, `"name":"lookup","parameters":{"properties":{"query":{"type":"string"}},"type":"object"}`)
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
