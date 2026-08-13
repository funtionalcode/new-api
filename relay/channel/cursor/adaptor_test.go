package cursor

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCursorTestContext(t *testing.T) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	return c
}

func setCursorTestAgentHeaders(c *gin.Context, agentID string) {
	c.Request.Header.Set(cursorAgentIDHeader, agentID)
	c.Request.Header.Set(cursorAgentSignatureHeader, cursorAgentSignature(c.GetInt("id"), agentID))
}

func TestCursorAdaptorUsesCloudAgentsEndpoint(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeCursor,
		ChannelBaseUrl: "https://api.cursor.com",
		ApiKey:         "cursor-secret",
	}}

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://api.cursor.com/v1/agents", requestURL)

	header := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(newCursorTestContext(t), &header, info))
	assert.Equal(t, "Bearer cursor-secret", header.Get("Authorization"))
	assert.Equal(t, "application/json", header.Get("Content-Type"))
}

func TestCursorAdaptorNormalizesTrailingSlashInBaseURL(t *testing.T) {
	requestURL, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeCursor,
		ChannelBaseUrl: "https://api.cursor.com/",
	}})

	require.NoError(t, err)
	assert.Equal(t, "https://api.cursor.com/v1/agents", requestURL)
}

func TestCursorAdaptorRejectsNonChatEndpoint(t *testing.T) {
	_, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCursor,
			ChannelBaseUrl: "https://api.cursor.com",
		},
	})

	require.ErrorContains(t, err, "only /v1/chat/completions is supported")
}

func TestConvertOpenAIRequestPreservesFullConversation(t *testing.T) {
	adaptor := &Adaptor{}
	c := newCursorTestContext(t)
	request := &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{
			{Role: "system", Content: "Answer concisely."},
			{Role: "user", Content: "My name is Ada."},
			{Role: "assistant", Content: "Hello Ada."},
			{Role: "user", Content: "What is my name?"},
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(c, nil, request)
	require.NoError(t, err)
	createRequest, ok := converted.(*createAgentRequest)
	require.True(t, ok)
	assert.Equal(t, "composer-2", createRequest.Model.ID)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"system","content":"Answer concisely."}`)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"assistant","content":"Hello Ada."}`)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"user","content":"What is my name?"}`)
	assert.Contains(t, createRequest.Prompt.Text, "Return only the assistant reply")
}

func TestConvertOpenAIRequestUsesMinimalChannelTestPrompt(t *testing.T) {
	info := &relaycommon.RelayInfo{IsChannelTest: true}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(newCursorTestContext(t), info, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	})

	require.NoError(t, err)
	createRequest, ok := converted.(*createAgentRequest)
	require.True(t, ok)
	assert.Equal(t, "Reply exactly with OK. Do not use tools or inspect any repository.", createRequest.Prompt.Text)
	assert.Equal(t, "new-api channel test", createRequest.Name)
}

func TestConvertOpenAIRequestContinuesPersistentAgentWithLatestUserMessage(t *testing.T) {
	adaptor := &Adaptor{}
	c := newCursorTestContext(t)
	setCursorTestAgentHeaders(c, "bc-00000000-0000-0000-0000-000000000001")
	request := &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{
			{Role: "user", Content: "Remember this."},
			{Role: "assistant", Content: "I will."},
			{Role: "user", Content: "What did I ask?"},
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(c, nil, request)
	require.NoError(t, err)
	runRequest, ok := converted.(*createRunRequest)
	require.True(t, ok)
	assert.Equal(t, "What did I ask?", runRequest.Prompt.Text)
	assert.Equal(t, "bc-00000000-0000-0000-0000-000000000001", c.GetString(cursorAgentIDContextKey))
}

func TestCursorPersistentHeadersRoundTripForSameUser(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	responseRecorder := httptest.NewRecorder()
	responseContext, _ := gin.CreateTestContext(responseRecorder)
	responseContext.Set("id", 7)
	setCursorAgentResponseHeaders(responseContext, agentID)

	requestContext := newCursorTestContext(t)
	requestContext.Set("id", 7)
	requestContext.Request.Header.Set(cursorAgentIDHeader, responseRecorder.Header().Get(cursorAgentIDHeader))
	requestContext.Request.Header.Set(cursorAgentSignatureHeader, responseRecorder.Header().Get(cursorAgentSignatureHeader))

	require.NoError(t, validateCursorAgentID(requestContext, agentID))
}

func TestConvertOpenAIRequestRejectsPersistentAgentFromAnotherUser(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)
	c.Set("id", 2)

	_, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
	})

	require.ErrorContains(t, err, "does not belong to this user")
}

func TestConvertOpenAIRequestRejectsUnknownPersistentSignatureVersion(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)
	c.Request.Header.Set(cursorAgentSignatureHeader, "v2.invalid")

	_, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
	})

	require.ErrorContains(t, err, "signature version is invalid")
}

func TestConvertOpenAIRequestRejectsInvalidPersistentMetadataTypes(t *testing.T) {
	c := newCursorTestContext(t)

	_, err := (&Adaptor{}).ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
		Metadata: []byte(`{"cursor_persistent":"true"}`),
	})

	require.ErrorContains(t, err, "metadata.cursor_persistent must be a boolean")
}

func TestConvertOpenAIRequestRejectsNonTextAndTools(t *testing.T) {
	adaptor := &Adaptor{}
	c := newCursorTestContext(t)

	_, err := adaptor.ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: []any{map[string]any{"type": "image_url"}}}},
	})
	require.ErrorContains(t, err, "text-only")

	_, err = adaptor.ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
		Tools:    []dto.ToolCallRequest{{}},
	})
	require.ErrorContains(t, err, "tools are not supported")
}

func TestConvertOpenAIRequestAcceptsTextContentParts(t *testing.T) {
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(newCursorTestContext(t), nil, &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "Hello "},
				map[string]any{"type": "text", "text": "Cursor"},
			},
		}},
	})

	require.NoError(t, err)
	createRequest := converted.(*createAgentRequest)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"user","content":"Hello Cursor"}`)
}

func TestScanCursorEvents(t *testing.T) {
	input := strings.Join([]string{
		"event: status",
		`data: {"runId":"run-1","status":"RUNNING"}`,
		"",
		"event: assistant",
		`data: {"text":"Hello"}`,
		"",
		"event: result",
		`data: {"runId":"run-1","status":"FINISHED","text":"Hello"}`,
		"",
	}, "\n")

	var events []cursorEvent
	err := scanCursorEvents(strings.NewReader(input), func(event cursorEvent) error {
		events = append(events, event)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "assistant", events[1].Type)
	assert.JSONEq(t, `{"text":"Hello"}`, string(events[1].Data))
}

func TestCursorUsageToOpenAIUsageIncludesCacheTokens(t *testing.T) {
	usage := cursorUsageToOpenAI(cursorUsage{
		InputTokens:      10,
		OutputTokens:     5,
		CacheWriteTokens: 3,
		CacheReadTokens:  7,
		TotalTokens:      25,
	}, nil)

	assert.Equal(t, 20, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
	assert.Equal(t, 25, usage.TotalTokens)
	assert.Equal(t, 7, usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 3, usage.PromptTokensDetails.CacheWriteTokens)
}

func TestCursorUsageToOpenAIClampsInvalidUpstreamCounts(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	usage := cursorUsageToOpenAI(cursorUsage{
		InputTokens:      -1,
		OutputTokens:     math.MaxInt64,
		CacheWriteTokens: 2,
		CacheReadTokens:  3,
	}, info)

	assert.Zero(t, usage.PromptTokensDetails.TextTokens)
	assert.Equal(t, common.MaxQuota, usage.CompletionTokens)
	assert.Equal(t, common.MaxQuota, usage.TotalTokens)
	require.NotNil(t, info.QuotaClamp)
	assert.Equal(t, common.QuotaClampOverflow, info.QuotaClamp.Kind)
}

func TestCursorNonStreamResponseUsesFinalResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-test")

	body := strings.Join([]string{
		"event: assistant",
		`data: {"text":"Hel"}`,
		"",
		"event: assistant",
		`data: {"text":"lo"}`,
		"",
		"event: result",
		`data: {"runId":"run-1","status":"FINISHED","text":"Hello"}`,
		"",
		"event: done",
		"data: {}",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			cursorClientStreamHeader:    []string{"false"},
			cursorSkipRemoteUsageHeader: []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"}}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "chatcmpl-req-cursor-test", response.Id)
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, "composer-2", response.Model)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "assistant", response.Choices[0].Message.Role)
	assert.Equal(t, "Hello", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", response.Choices[0].FinishReason)
	assert.Positive(t, response.Usage.CompletionTokens)
	assert.Equal(t, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestCursorAdaptorRunsCloudAgentAndDeletesEphemeralSession(t *testing.T) {
	var mutex sync.Mutex
	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		mutex.Unlock()
		assert.Equal(t, "Bearer cursor-secret", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents":
			var request createAgentRequest
			assert.NoError(t, common.DecodeJson(r.Body, &request))
			assert.Equal(t, "composer-2", request.Model.ID)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"agent":{"id":"bc-00000000-0000-0000-0000-000000000001"},"run":{"id":"run-1"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stream"):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: assistant\ndata: {\"text\":\"Hello\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"Hello\"}\n\nevent: done\ndata: {}\n\n"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage"):
			assert.Equal(t, "run-1", r.URL.Query().Get("runId"))
			_, _ = w.Write([]byte(`{"totalUsage":{"inputTokens":10,"outputTokens":5,"cacheWriteTokens":3,"cacheReadTokens":7,"totalTokens":25},"runs":[{"id":"run-1","usage":{"inputTokens":10,"outputTokens":5,"cacheWriteTokens":3,"cacheReadTokens":7,"totalTokens":25}}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/agents/bc-00000000-0000-0000-0000-000000000001":
			_, _ = w.Write([]byte(`{"id":"bc-00000000-0000-0000-0000-000000000001"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, common.RequestIdKey, "cursor-e2e")
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeCursor,
			ChannelBaseUrl:    server.URL,
			ApiKey:            "cursor-secret",
			UpstreamModelName: "composer-2",
		},
	}
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "Hello"}},
	})
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)

	upstream, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	response := upstream.(*http.Response)
	assert.Equal(t, cursorEventStreamContentType, response.Header.Get("Content-Type"))
	usageValue, apiErr := adaptor.DoResponse(c, response, info)
	require.Nil(t, apiErr)
	usage := usageValue.(*dto.Usage)
	assert.Equal(t, 20, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, []string{
		"POST /v1/agents",
		"GET /v1/agents/bc-00000000-0000-0000-0000-000000000001/runs/run-1/stream",
		"GET /v1/agents/bc-00000000-0000-0000-0000-000000000001/usage?runId=run-1",
		"DELETE /v1/agents/bc-00000000-0000-0000-0000-000000000001",
	}, requests)
}

func TestCursorAdaptorContinuesPersistentAgentWithoutDeletingSession(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	var mutex sync.Mutex
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		mutex.Unlock()

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents/"+agentID+"/runs":
			var request createRunRequest
			assert.NoError(t, common.DecodeJson(r.Body, &request))
			assert.Equal(t, "What did I ask?", request.Prompt.Text)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"run":{"id":"run-2"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stream"):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: result\ndata: {\"runId\":\"run-2\",\"status\":\"FINISHED\",\"text\":\"You asked me to remember this.\"}\n\n"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage"):
			_, _ = w.Write([]byte(`{"totalUsage":{"inputTokens":8,"outputTokens":7,"cacheWriteTokens":0,"cacheReadTokens":4,"totalTokens":19},"runs":[{"id":"run-2","usage":{"inputTokens":8,"outputTokens":7,"cacheWriteTokens":0,"cacheReadTokens":4,"totalTokens":19}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	setCursorTestAgentHeaders(c, agentID)
	common.SetContextKey(c, common.RequestIdKey, "cursor-persistent")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:       constant.ChannelTypeCursor,
		ChannelBaseUrl:    server.URL,
		ApiKey:            "cursor-secret",
		UpstreamModelName: "composer-2",
	}}
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{
			{Role: "user", Content: "Remember this."},
			{Role: "assistant", Content: "I will."},
			{Role: "user", Content: "What did I ask?"},
		},
	})
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)

	upstream, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	usageValue, apiErr := adaptor.DoResponse(c, upstream.(*http.Response), info)
	require.Nil(t, apiErr)
	assert.Equal(t, 12, usageValue.(*dto.Usage).PromptTokens)
	assert.Equal(t, agentID, recorder.Header().Get(cursorAgentIDHeader))

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, []string{
		"POST /v1/agents/" + agentID + "/runs",
		"GET /v1/agents/" + agentID + "/runs/run-2/stream",
		"GET /v1/agents/" + agentID + "/usage?runId=run-2",
	}, requests)
}

func TestCursorAdaptorOnlyAcceptsOpenAITextRequests(t *testing.T) {
	adaptor := &Adaptor{}

	_, err := adaptor.ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{})
	require.ErrorContains(t, err, "/v1/responses endpoint not supported")
	_, err = adaptor.ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{})
	require.ErrorContains(t, err, "/v1/messages endpoint not supported")
	_, err = adaptor.ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
	require.ErrorContains(t, err, "Gemini endpoint not supported")
}
