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
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
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
	common.SetContextKey(c, constant.ContextKeyChannelId, 42)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	return c
}

func setCursorTestAgentHeaders(c *gin.Context, agentID string) {
	c.Request.Header.Set(constant.CursorAgentIDHeader, agentID)
	c.Request.Header.Set(constant.CursorAgentSignatureHeader, cursorAgentSignature(c.GetInt("id"), agentID, 42, 0, "cursor-secret"))
	c.Request.Header.Set(constant.CursorAgentChannelIDHeader, "42")
	c.Request.Header.Set(constant.CursorAgentKeyIndexHeader, "0")
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

func TestCursorAdaptorUsesCloudAgentsEndpointForClaudeMessages(t *testing.T) {
	requestURL, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCursor,
			ChannelBaseUrl: "https://api.cursor.com",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://api.cursor.com/v1/agents", requestURL)
}

func TestCursorAdaptorUsesCloudAgentsEndpointForResponses(t *testing.T) {
	requestURL, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCursor,
			ChannelBaseUrl: "https://api.cursor.com",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://api.cursor.com/v1/agents", requestURL)

	_, err = (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCursor,
			ChannelBaseUrl: "https://api.cursor.com",
		},
	})
	require.ErrorContains(t, err, "/v1/responses/compact endpoint not supported")
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

	converted, err := adaptor.ConvertOpenAIRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ApiKey: "cursor-secret"}}, request)
	require.NoError(t, err)
	runRequest, ok := converted.(*createRunRequest)
	require.True(t, ok)
	assert.Equal(t, "What did I ask?", runRequest.Prompt.Text)
	assert.Equal(t, "bc-00000000-0000-0000-0000-000000000001", c.GetString(cursorAgentIDContextKey))
}

func TestConvertOpenAIRequestTreatsCleanupCommandAsAgentDeletion(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ApiKey: "cursor-secret"}}, &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{
			{Role: "user", Content: "previous question"},
			{Role: "assistant", Content: "previous answer"},
			{Role: "user", Content: "  清理会话agent  "},
		},
	})

	require.NoError(t, err)
	assert.IsType(t, &deleteAgentRequest{}, converted)
	assert.Equal(t, constant.CursorAgentLifecycleDelete, common.GetContextKeyString(c, constant.ContextKeyCursorAgentLifecycle))
}

func TestConvertOpenAIRequestRejectsCleanupWithoutActiveAgent(t *testing.T) {
	_, err := (&Adaptor{}).ConvertOpenAIRequest(newCursorTestContext(t), nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "清理会话agent"}},
	})

	require.ErrorContains(t, err, "no active Cursor Agent")
}

func TestCursorPersistentHeadersRoundTripForSameUser(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	responseRecorder := httptest.NewRecorder()
	responseContext, _ := gin.CreateTestContext(responseRecorder)
	responseContext.Set("id", 7)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ChannelMultiKeyIndex: 0, ApiKey: "cursor-secret"}}
	setCursorAgentResponseHeaders(responseContext, info, agentID)

	requestContext := newCursorTestContext(t)
	requestContext.Set("id", 7)
	requestContext.Request.Header.Set(constant.CursorAgentIDHeader, responseRecorder.Header().Get(constant.CursorAgentIDHeader))
	requestContext.Request.Header.Set(constant.CursorAgentSignatureHeader, responseRecorder.Header().Get(constant.CursorAgentSignatureHeader))

	require.NoError(t, validateCursorAgentID(requestContext, info, agentID))
}

func TestConvertOpenAIRequestRejectsPersistentAgentFromAnotherUser(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)
	c.Set("id", 2)

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ApiKey: "cursor-secret"}}, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
	})

	require.NoError(t, err)
	assert.IsType(t, &createAgentRequest{}, converted)
	assert.Empty(t, c.GetString(cursorAgentIDContextKey))
}

func TestConvertOpenAIRequestRejectsUnknownPersistentSignatureVersion(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)
	c.Request.Header.Set(constant.CursorAgentSignatureHeader, "v3.invalid")

	_, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ApiKey: "cursor-secret"}}, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
	})

	require.ErrorContains(t, err, "signature version is invalid")
}

func TestConvertOpenAIRequestRenewsLegacyPersistentAgentWithoutReusingIt(t *testing.T) {
	c := newCursorTestContext(t)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)
	c.Request.Header.Set(constant.CursorAgentSignatureHeader, "v1.old-instance-signature")

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42, ApiKey: "cursor-secret"}}, &dto.GeneralOpenAIRequest{
		Model: "composer-2",
		Messages: []dto.Message{
			{Role: "user", Content: "Remember this."},
			{Role: "assistant", Content: "I will."},
			{Role: "user", Content: "Continue."},
		},
	})

	require.NoError(t, err)
	createRequest, ok := converted.(*createAgentRequest)
	require.True(t, ok)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"assistant","content":"I will."}`)
	assert.Empty(t, c.GetString(cursorAgentIDContextKey))
	assert.True(t, c.GetBool(cursorPersistentContextKey))
}

func TestCursorAgentSignatureIsStableAcrossRuntimeSecretChanges(t *testing.T) {
	originalSecret := common.CryptoSecret
	t.Cleanup(func() { common.CryptoSecret = originalSecret })
	agentID := "bc-00000000-0000-0000-0000-000000000001"

	common.CryptoSecret = "first-instance-secret"
	first := cursorAgentSignature(7, agentID, 42, 0, "cursor-secret")
	common.CryptoSecret = "second-instance-secret"
	second := cursorAgentSignature(7, agentID, 42, 0, "cursor-secret")

	assert.Equal(t, first, second)
	assert.True(t, strings.HasPrefix(first, "v2."))
}

func TestCursorAgentSignatureBindsUserAndChannelKey(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"

	first := cursorAgentSignature(7, agentID, 42, 0, "first-cursor-key")

	assert.NotEqual(t, first, cursorAgentSignature(8, agentID, 42, 0, "first-cursor-key"))
	assert.NotEqual(t, first, cursorAgentSignature(7, agentID, 43, 0, "first-cursor-key"))
	assert.NotEqual(t, first, cursorAgentSignature(7, agentID, 42, 1, "first-cursor-key"))
	assert.NotEqual(t, first, cursorAgentSignature(7, agentID, 42, 0, "second-cursor-key"))
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

func TestConvertOpenAIRequestRejectsNonTextAndAcceptsTools(t *testing.T) {
	adaptor := &Adaptor{}
	c := newCursorTestContext(t)

	_, err := adaptor.ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: []any{map[string]any{"type": "image_url"}}}},
	})
	require.ErrorContains(t, err, "text-only")

	converted, err := adaptor.ConvertOpenAIRequest(c, nil, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "lookup_weather",
				Description: "Look up weather",
				Parameters:  map[string]any{"type": "object"},
			},
		}},
	})
	require.NoError(t, err)
	createRequest := converted.(*createAgentRequest)
	assert.Contains(t, createRequest.Prompt.Text, `"cursor_external_tool_calls"`)
	assert.Contains(t, createRequest.Prompt.Text, `"name":"lookup_weather"`)
	assert.Contains(t, createRequest.Prompt.Text, `"description":"Look up weather"`)
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

func TestConvertClaudeRequestBuildsCursorAgentConversation(t *testing.T) {
	persistent := true
	c := newCursorTestContext(t)
	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, nil, &dto.ClaudeRequest{
		Model:  "claude-opus-5",
		System: "Answer concisely.",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "My name is Ada."},
			{Role: "assistant", Content: "Hello Ada."},
			{Role: "user", Content: "What is my name?"},
		},
		Stream:       &persistent,
		Metadata:     []byte(`{"cursor_persistent":true}`),
		OutputConfig: []byte(`{"effort":"high"}`),
	})

	require.NoError(t, err)
	createRequest, ok := converted.(*createAgentRequest)
	require.True(t, ok)
	assert.Equal(t, "claude-opus-5", createRequest.Model.ID)
	assert.Equal(t, []cursorModelParam{{ID: "thinking", Value: "high"}}, createRequest.Model.Params)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"system","content":"Answer concisely."}`)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"assistant","content":"Hello Ada."}`)
	assert.Contains(t, createRequest.Prompt.Text, `{"role":"user","content":"What is my name?"}`)
	assert.True(t, c.GetBool(cursorPersistentContextKey))
}

func TestConvertClaudeRequestContinuesPersistentAgent(t *testing.T) {
	c := newCursorTestContext(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId: 42,
		ApiKey:    "cursor-secret",
	}}, &dto.ClaudeRequest{
		Model: "claude-opus-5",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "Remember this."},
			{Role: "assistant", Content: "I will."},
			{Role: "user", Content: "What did I ask?"},
		},
	})

	require.NoError(t, err)
	runRequest, ok := converted.(*createRunRequest)
	require.True(t, ok)
	assert.Equal(t, "What did I ask?", runRequest.Prompt.Text)
	assert.Equal(t, agentID, c.GetString(cursorAgentIDContextKey))
}

func TestConvertClaudeRequestPreservesToolHistoryForPersistentAgent(t *testing.T) {
	c := newCursorTestContext(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	setCursorTestAgentHeaders(c, agentID)

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId: 42,
		ApiKey:    "cursor-secret",
	}}, &dto.ClaudeRequest{
		Model: "claude-opus-5",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "What is the weather?"},
			{Role: "assistant", Content: []any{map[string]any{
				"type": "tool_use", "id": "toolu_1", "name": "lookup_weather", "input": map[string]any{"city": "Paris"},
			}}},
			{Role: "user", Content: []any{map[string]any{
				"type": "tool_result", "tool_use_id": "toolu_1", "content": "15 C",
			}}},
		},
		Tools: []any{map[string]any{
			"name":         "lookup_weather",
			"description":  "Look up weather",
			"input_schema": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		}},
	})

	require.NoError(t, err)
	runRequest, ok := converted.(*createRunRequest)
	require.True(t, ok)
	assert.Contains(t, runRequest.Prompt.Text, `"role":"tool"`)
	assert.Contains(t, runRequest.Prompt.Text, `"tool_call_id":"toolu_1"`)
	assert.Contains(t, runRequest.Prompt.Text, `"content":"15 C"`)
	assert.Contains(t, runRequest.Prompt.Text, `"name":"lookup_weather"`)
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

func TestCursorNonStreamResponseUsesClaudeMessagesFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-claude")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			cursorClientStreamHeader:    []string{"false"},
			cursorSkipRemoteUsageHeader: []string{"true"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: assistant",
			`data: {"text":"Hello from Cursor"}`,
			"",
			"event: result",
			`data: {"runId":"run-1","status":"FINISHED","text":"Hello from Cursor"}`,
			"",
		}, "\n"))),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	var response dto.ClaudeResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "message", response.Type)
	assert.Equal(t, "assistant", response.Role)
	assert.Equal(t, "claude-opus-5", response.Model)
	require.Len(t, response.Content, 1)
	assert.Equal(t, "Hello from Cursor", response.Content[0].GetText())
	assert.Equal(t, "end_turn", response.StopReason)
	require.NotNil(t, response.Usage)
	assert.Positive(t, response.Usage.OutputTokens)
}

func TestCursorStreamResponseUsesClaudeMessagesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-claude-stream")

	responseHeader := make(http.Header)
	responseHeader.Set(cursorClientStreamHeader, "true")
	responseHeader.Set(cursorSkipRemoteUsageHeader, "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: thinking",
			`data: {"text":"Check context. "}`,
			"",
			"event: assistant",
			`data: {"text":"Hello "}`,
			"",
			"event: assistant",
			`data: {"text":"from Cursor"}`,
			"",
			"event: result",
			`data: {"runId":"run-1","status":"FINISHED","text":"Hello from Cursor"}`,
			"",
		}, "\n"))),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, 1, strings.Count(body, "event: message_start\n"))
	assert.Contains(t, body, `"type":"thinking_delta","thinking":"Check context. "`)
	assert.Contains(t, body, `"type":"text_delta","text":"Hello "`)
	assert.Contains(t, body, `"type":"text_delta","text":"from Cursor"`)
	assert.Equal(t, 1, strings.Count(body, "event: message_delta\n"))
	assert.Equal(t, 1, strings.Count(body, "event: message_stop\n"))
	assert.NotContains(t, body, "[DONE]")
	assert.Less(t, strings.Index(body, "event: message_start\n"), strings.Index(body, "event: message_stop\n"))
}

func TestCursorNonStreamResponseConvertsExternalToolCallToClaude(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"lookup_weather": {Kind: "function", Name: "lookup_weather"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-claude-tool")

	toolEnvelope := `{"cursor_external_tool_calls":[{"id":"toolu_cursor_1","name":"lookup_weather","arguments":{"city":"Paris"}}]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			cursorClientStreamHeader:    []string{"false"},
			cursorSkipRemoteUsageHeader: []string{"true"},
		},
		Body: io.NopCloser(strings.NewReader("event: assistant\ndata: {\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	var response dto.ClaudeResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "tool_use", response.StopReason)
	require.Len(t, response.Content, 1)
	assert.Equal(t, "tool_use", response.Content[0].Type)
	assert.Equal(t, "toolu_cursor_1", response.Content[0].Id)
	assert.Equal(t, "lookup_weather", response.Content[0].Name)
	toolInput, ok := response.Content[0].Input.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Paris", toolInput["city"])
	assert.NotContains(t, recorder.Body.String(), "cursor_external_tool_calls")
}

func TestCursorStreamResponseConvertsExternalToolCallToClaudeEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"lookup_weather": {Kind: "function", Name: "lookup_weather"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-claude-tool-stream")

	toolEnvelope := `{"cursor_external_tool_calls":[{"id":"toolu_cursor_1","name":"lookup_weather","arguments":{"city":"Paris"}}]}`
	responseHeader := make(http.Header)
	responseHeader.Set(cursorClientStreamHeader, "true")
	responseHeader.Set(cursorSkipRemoteUsageHeader, "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body:       io.NopCloser(strings.NewReader("event: assistant\ndata: {\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	assert.Contains(t, body, `"type":"tool_use","id":"toolu_cursor_1","name":"lookup_weather"`)
	assert.Contains(t, body, `"type":"input_json_delta","partial_json":"{\"city\":\"Paris\"}"`)
	assert.Contains(t, body, `"stop_reason":"tool_use"`)
	assert.Equal(t, 1, strings.Count(body, "event: message_stop\n"))
	assert.NotContains(t, body, "cursor_external_tool_calls")
}

func TestCursorStreamResponseKeepsNormalTextStreamingWhenToolsAreAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"lookup_weather": {Kind: "function", Name: "lookup_weather"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-claude-text-with-tools")

	responseHeader := make(http.Header)
	responseHeader.Set(cursorClientStreamHeader, "true")
	responseHeader.Set(cursorSkipRemoteUsageHeader, "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: assistant",
			`data: {"text":"Hello "}`,
			"",
			"event: assistant",
			`data: {"text":"without a tool"}`,
			"",
			"event: result",
			`data: {"runId":"run-1","status":"FINISHED","text":"Hello without a tool"}`,
			"",
		}, "\n"))),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	assert.Contains(t, body, `"type":"text_delta","text":"Hello "`)
	assert.Contains(t, body, `"type":"text_delta","text":"without a tool"`)
	assert.Equal(t, 1, strings.Count(body, "event: message_stop\n"))
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
			assert.Equal(t, []cursorModelParam{{ID: "thinking", Value: "high"}}, request.Model.Params)
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
		Model:           "composer-2",
		Messages:        []dto.Message{{Role: "user", Content: "Hello"}},
		ReasoningEffort: "high",
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
		ChannelId:         42,
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
	assert.Equal(t, agentID, recorder.Header().Get(constant.CursorAgentIDHeader))

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, []string{
		"POST /v1/agents/" + agentID + "/runs",
		"GET /v1/agents/" + agentID + "/runs/run-2/stream",
		"GET /v1/agents/" + agentID + "/usage?runId=run-2",
	}, requests)
}

func TestCursorAdaptorDeletesPersistentAgentWithoutStartingRun(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	requests := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		assert.Equal(t, "Bearer cursor-secret", r.Header.Get("Authorization"))
		if r.Method == http.MethodDelete && r.URL.Path == "/v1/agents/"+agentID {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	common.SetContextKey(c, constant.ContextKeyChannelId, 42)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	common.SetContextKey(c, common.RequestIdKey, "cursor-delete")
	setCursorTestAgentHeaders(c, agentID)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId:         42,
		ChannelType:       constant.ChannelTypeCursor,
		ChannelBaseUrl:    server.URL,
		ApiKey:            "cursor-secret",
		UpstreamModelName: "composer-2",
	}}
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "清理会话agent"}},
	})
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)

	upstream, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	usageValue, apiErr := adaptor.DoResponse(c, upstream.(*http.Response), info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageValue)
	assert.Equal(t, "true", recorder.Header().Get(constant.CursorAgentDeletedHeader))
	assert.Equal(t, []string{"DELETE /v1/agents/" + agentID}, requests)
	assert.Contains(t, recorder.Body.String(), "Cursor Agent deleted.")
}

func TestCursorAdaptorReusesServerManagedClaudeCodeAgentUntilCleanup(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000099"
	var mutex sync.Mutex
	requests := make([]string, 0, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		mutex.Unlock()

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"agent":{"id":"` + agentID + `"},"run":{"id":"run-1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents/"+agentID+"/runs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"run":{"id":"run-2"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/runs/run-1/stream"):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"first reply\"}\n\n"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/runs/run-2/stream"):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: result\ndata: {\"runId\":\"run-2\",\"status\":\"FINISHED\",\"text\":\"second reply\"}\n\n"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage") && r.URL.Query().Get("runId") == "run-1":
			_, _ = w.Write([]byte(`{"runs":[{"id":"run-1","usage":{"inputTokens":10,"outputTokens":2,"cacheWriteTokens":8,"cacheReadTokens":0,"totalTokens":20}}]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage") && r.URL.Query().Get("runId") == "run-2":
			_, _ = w.Write([]byte(`{"runs":[{"id":"run-2","usage":{"inputTokens":3,"outputTokens":2,"cacheWriteTokens":0,"cacheReadTokens":9,"totalTokens":14}}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/agents/"+agentID:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            42,
			ChannelType:          constant.ChannelTypeCursor,
			ChannelBaseUrl:       server.URL,
			ChannelMultiKeyIndex: 0,
			ApiKey:               "cursor-secret",
			UpstreamModelName:    "claude-sonnet-5",
		},
	}
	adaptor := &Adaptor{}

	firstRecorder := httptest.NewRecorder()
	first, _ := gin.CreateTestContext(firstRecorder)
	first.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	first.Set("id", 23)
	common.SetContextKey(first, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(first, common.RequestIdKey, "cursor-claude-first")
	firstStorage, err := common.CreateBodyStorage([]byte(`{"model":"claude-sonnet-5","metadata":{"user_id":"claude-session-99"}}`))
	require.NoError(t, err)
	t.Cleanup(func() { _ = firstStorage.Close() })
	first.Set(common.KeyBodyStorage, firstStorage)
	service.PrepareCursorAgentSession(first, "claude-sonnet-5")
	firstConverted, err := adaptor.ConvertClaudeRequest(first, info, &dto.ClaudeRequest{
		Model:    "claude-sonnet-5",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "first question"}},
		Metadata: []byte(`{"user_id":"claude-session-99"}`),
	})
	require.NoError(t, err)
	firstBody, err := common.Marshal(firstConverted)
	require.NoError(t, err)
	firstUpstream, err := adaptor.DoRequest(first, info, bytes.NewReader(firstBody))
	require.NoError(t, err)
	firstUsageValue, firstAPIErr := adaptor.DoResponse(first, firstUpstream.(*http.Response), info)
	require.Nil(t, firstAPIErr)
	firstUsage := firstUsageValue.(*dto.Usage)
	assert.Equal(t, 8, firstUsage.PromptTokensDetails.CacheWriteTokens)
	assert.Equal(t, constant.CursorAgentLifecycleCreate, common.GetContextKeyString(first, constant.ContextKeyCursorAgentLifecycle))

	secondRecorder := httptest.NewRecorder()
	second, _ := gin.CreateTestContext(secondRecorder)
	second.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	second.Set("id", 23)
	common.SetContextKey(second, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(second, common.RequestIdKey, "cursor-claude-second")
	secondStorage, err := common.CreateBodyStorage([]byte(`{"model":"claude-sonnet-5","metadata":{"user_id":"claude-session-99"}}`))
	require.NoError(t, err)
	t.Cleanup(func() { _ = secondStorage.Close() })
	second.Set(common.KeyBodyStorage, secondStorage)
	service.PrepareCursorAgentSession(second, "claude-sonnet-5")
	assert.Equal(t, agentID, second.GetHeader(constant.CursorAgentIDHeader))
	secondConverted, err := adaptor.ConvertClaudeRequest(second, info, &dto.ClaudeRequest{
		Model: "claude-sonnet-5",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "first reply"},
			{Role: "user", Content: "second question"},
		},
		Metadata: []byte(`{"user_id":"claude-session-99"}`),
	})
	require.NoError(t, err)
	assert.IsType(t, &createRunRequest{}, secondConverted)
	secondBody, err := common.Marshal(secondConverted)
	require.NoError(t, err)
	secondUpstream, err := adaptor.DoRequest(second, info, bytes.NewReader(secondBody))
	require.NoError(t, err)
	secondUsageValue, secondAPIErr := adaptor.DoResponse(second, secondUpstream.(*http.Response), info)
	require.Nil(t, secondAPIErr)
	secondUsage := secondUsageValue.(*dto.Usage)
	assert.Equal(t, 9, secondUsage.PromptTokensDetails.CachedTokens)
	assert.Empty(t, common.GetContextKeyString(second, constant.ContextKeyCursorAgentLifecycle))

	cleanupRecorder := httptest.NewRecorder()
	cleanup, _ := gin.CreateTestContext(cleanupRecorder)
	cleanup.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	cleanup.Set("id", 23)
	common.SetContextKey(cleanup, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(cleanup, common.RequestIdKey, "cursor-claude-cleanup")
	cleanupStorage, err := common.CreateBodyStorage([]byte(`{"model":"claude-sonnet-5","metadata":{"user_id":"claude-session-99"}}`))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanupStorage.Close() })
	cleanup.Set(common.KeyBodyStorage, cleanupStorage)
	service.PrepareCursorAgentSession(cleanup, "claude-sonnet-5")
	cleanupConverted, err := adaptor.ConvertClaudeRequest(cleanup, info, &dto.ClaudeRequest{
		Model:    "claude-sonnet-5",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: cursorCleanupCommand}},
		Metadata: []byte(`{"user_id":"claude-session-99"}`),
	})
	require.NoError(t, err)
	cleanupBody, err := common.Marshal(cleanupConverted)
	require.NoError(t, err)
	cleanupUpstream, err := adaptor.DoRequest(cleanup, info, bytes.NewReader(cleanupBody))
	require.NoError(t, err)
	_, cleanupAPIErr := adaptor.DoResponse(cleanup, cleanupUpstream.(*http.Response), info)
	require.Nil(t, cleanupAPIErr)
	assert.Equal(t, constant.CursorAgentLifecycleDelete, common.GetContextKeyString(cleanup, constant.ContextKeyCursorAgentLifecycle))

	afterCleanup := newCursorTestContext(t)
	afterCleanup.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	afterCleanup.Set("id", 23)
	common.SetContextKey(afterCleanup, constant.ContextKeyUsingGroup, "default")
	afterCleanupStorage, err := common.CreateBodyStorage([]byte(`{"model":"claude-sonnet-5","metadata":{"user_id":"claude-session-99"}}`))
	require.NoError(t, err)
	t.Cleanup(func() { _ = afterCleanupStorage.Close() })
	afterCleanup.Set(common.KeyBodyStorage, afterCleanupStorage)
	service.PrepareCursorAgentSession(afterCleanup, "claude-sonnet-5")
	assert.Empty(t, afterCleanup.GetHeader(constant.CursorAgentIDHeader))

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, []string{
		"POST /v1/agents",
		"GET /v1/agents/" + agentID + "/runs/run-1/stream",
		"GET /v1/agents/" + agentID + "/usage?runId=run-1",
		"POST /v1/agents/" + agentID + "/runs",
		"GET /v1/agents/" + agentID + "/runs/run-2/stream",
		"GET /v1/agents/" + agentID + "/usage?runId=run-2",
		"DELETE /v1/agents/" + agentID,
	}, requests)
}

func TestCursorAdaptorFallsBackToRunWhenStreamIsUnavailable(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	runID := "run-00000000-0000-0000-0000-000000000001"
	requests := make([]string, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents/"+agentID+"/runs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"run":{"id":"` + runID + `"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stream"):
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(`{"error":{"code":"stream_unavailable","message":"Run stream is no longer available"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/"+agentID+"/runs/"+runID:
			_, _ = w.Write([]byte(`{"id":"` + runID + `","agentId":"` + agentID + `","status":"FINISHED","result":"Fallback reply","createdAt":"2026-08-14T00:00:00Z","updatedAt":"2026-08-14T00:00:01Z"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage"):
			_, _ = w.Write([]byte(`{"totalUsage":{"inputTokens":8,"outputTokens":3,"cacheWriteTokens":0,"cacheReadTokens":2,"totalTokens":13},"runs":[{"id":"` + runID + `","usage":{"inputTokens":8,"outputTokens":3,"cacheWriteTokens":0,"cacheReadTokens":2,"totalTokens":13}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	common.SetContextKey(c, constant.ContextKeyChannelId, 42)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	common.SetContextKey(c, common.RequestIdKey, "cursor-stream-fallback")
	setCursorTestAgentHeaders(c, agentID)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId:         42,
		ChannelType:       constant.ChannelTypeCursor,
		ChannelBaseUrl:    server.URL,
		ApiKey:            "cursor-secret",
		UpstreamModelName: "composer-2",
	}}
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    "composer-2",
		Messages: []dto.Message{{Role: "user", Content: "continue"}},
	})
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)

	upstream, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	usageValue, apiErr := adaptor.DoResponse(c, upstream.(*http.Response), info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageValue)
	assert.Contains(t, recorder.Body.String(), "Fallback reply")
	assert.Equal(t, agentID, recorder.Header().Get(constant.CursorAgentIDHeader))
	assert.NotContains(t, requests, "DELETE /v1/agents/"+agentID)
}

func TestCursorResponseFallsBackWhenStreamEmitsUnavailableError(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000001"
	runID := "run-00000000-0000-0000-0000-000000000001"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agents/"+agentID+"/runs/"+runID:
			_, _ = w.Write([]byte(`{"id":"` + runID + `","agentId":"` + agentID + `","status":"FINISHED","result":"Recovered reply","createdAt":"2026-08-14T00:00:00Z","updatedAt":"2026-08-14T00:00:01Z"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage"):
			_, _ = w.Write([]byte(`{"totalUsage":{"inputTokens":5,"outputTokens":2,"cacheWriteTokens":0,"cacheReadTokens":0,"totalTokens":7},"runs":[{"id":"` + runID + `","usage":{"inputTokens":5,"outputTokens":2,"cacheWriteTokens":0,"cacheReadTokens":0,"totalTokens":7}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 1)
	common.SetContextKey(c, constant.ContextKeyChannelId, 42)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	common.SetContextKey(c, common.RequestIdKey, "cursor-stream-event-fallback")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelId:         42,
		ChannelType:       constant.ChannelTypeCursor,
		ChannelBaseUrl:    server.URL,
		ApiKey:            "cursor-secret",
		UpstreamModelName: "composer-2",
	}}
	responseHeader := make(http.Header)
	responseHeader.Set(cursorAgentIDInternalHeader, agentID)
	responseHeader.Set(cursorRunIDInternalHeader, runID)
	responseHeader.Set(cursorPersistentInternalKey, "true")
	responseHeader.Set(cursorClientStreamHeader, "false")
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body:       io.NopCloser(strings.NewReader("event: error\ndata: {\"code\":\"stream_unavailable\",\"message\":\"Run stream is no longer available\"}\n\n")),
	}

	usageValue, apiErr := (&Adaptor{}).DoResponse(c, response, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageValue)
	assert.Contains(t, recorder.Body.String(), "Recovered reply")
	assert.Equal(t, agentID, recorder.Header().Get(constant.CursorAgentIDHeader))
}

func TestConvertOpenAIResponsesRequestPreservesCodexCustomToolsAndHistory(t *testing.T) {
	c := newCursorTestContext(t)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}, dto.OpenAIResponsesRequest{
		Model: "composer-2",
		Input: []byte(`[
			{"role":"user","content":[{"type":"input_text","text":"Update the file"}]},
			{"type":"custom_tool_call","call_id":"call_patch","name":"apply_patch","input":"*** Begin Patch"},
			{"type":"custom_tool_call_output","call_id":"call_patch","output":"Done!"}
		]`),
		Tools: []byte(`[
			{"type":"custom","name":"apply_patch","description":"Apply a patch","format":{"type":"grammar","syntax":"lark","definition":"start: /.+/s"}},
			{"type":"function","name":"exec_command","description":"Run a command","parameters":{"type":"object"}}
		]`),
		Reasoning: &dto.Reasoning{Effort: "xhigh"},
	})
	require.NoError(t, err)

	createRequest, ok := converted.(*createAgentRequest)
	require.True(t, ok)
	assert.Equal(t, []cursorModelParam{{ID: "thinking", Value: "xhigh"}}, createRequest.Model.Params)
	assert.Contains(t, createRequest.Prompt.Text, `"type":"custom"`)
	assert.Contains(t, createRequest.Prompt.Text, `"name":"apply_patch"`)
	assert.Contains(t, createRequest.Prompt.Text, `"role":"tool"`)
	assert.Contains(t, createRequest.Prompt.Text, `"tool_call_id":"call_patch"`)
	toolKinds, ok := c.Get(cursorExternalToolsContextKey)
	require.True(t, ok)
	assert.Equal(t, map[string]cursorExternalToolSpec{
		"apply_patch":  {Kind: dto.CustomType, Name: "apply_patch"},
		"exec_command": {Kind: "function", Name: "exec_command"},
	}, toolKinds)
}

func TestConvertOpenAIResponsesRequestReusesServerManagedCodexAgent(t *testing.T) {
	agentID := "bc-00000000-0000-0000-0000-000000000098"
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            42,
			ChannelType:          constant.ChannelTypeCursor,
			ChannelMultiKeyIndex: 0,
			ApiKey:               "cursor-secret",
			UpstreamModelName:    "gpt-5.6-sol",
		},
	}
	requestBody := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"codex-session-98"}`)

	first := newCursorTestContext(t)
	first.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	first.Set("id", 24)
	common.SetContextKey(first, constant.ContextKeyUsingGroup, "default")
	firstStorage, err := common.CreateBodyStorage(requestBody)
	require.NoError(t, err)
	t.Cleanup(func() { _ = firstStorage.Close() })
	first.Set(common.KeyBodyStorage, firstStorage)
	service.PrepareCursorAgentSession(first, "gpt-5.6-sol")
	require.NoError(t, service.SaveCursorAgentSession(first, service.CursorAgentSession{
		AgentID:   agentID,
		Signature: cursorAgentSignature(24, agentID, 42, 0, "cursor-secret"),
		ChannelID: 42,
		KeyIndex:  0,
	}))

	second := newCursorTestContext(t)
	second.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	second.Set("id", 24)
	common.SetContextKey(second, constant.ContextKeyUsingGroup, "default")
	secondStorage, err := common.CreateBodyStorage(requestBody)
	require.NoError(t, err)
	t.Cleanup(func() { _ = secondStorage.Close() })
	second.Set(common.KeyBodyStorage, secondStorage)
	service.PrepareCursorAgentSession(second, "gpt-5.6-sol")

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(second, info, dto.OpenAIResponsesRequest{
		Model:          "gpt-5.6-sol",
		Input:          []byte(`[{"role":"user","content":[{"type":"input_text","text":"continue"}]}]`),
		PromptCacheKey: []byte(`"codex-session-98"`),
	})

	require.NoError(t, err)
	runRequest, ok := converted.(*createRunRequest)
	require.True(t, ok)
	assert.Equal(t, "continue", runRequest.Prompt.Text)
	assert.Equal(t, agentID, second.GetString(cursorAgentIDContextKey))
}

func TestCursorNonStreamResponseConvertsCustomToolCallToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"apply_patch": {Kind: dto.CustomType, Name: "apply_patch"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-responses-tool")

	toolEnvelope := `{"cursor_external_tool_calls":[{"id":"call_patch","name":"apply_patch","input":"*** Begin Patch"}]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			cursorClientStreamHeader:    []string{"false"},
			cursorSkipRemoteUsageHeader: []string{"true"},
		},
		Body: io.NopCloser(strings.NewReader("event: assistant\ndata: {\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Output, 1)
	assert.Equal(t, "custom_tool_call", response.Output[0].Type)
	assert.Equal(t, "call_patch", response.Output[0].CallId)
	assert.Equal(t, "apply_patch", response.Output[0].Name)
	require.NotNil(t, response.Output[0].Input)
	assert.Equal(t, "*** Begin Patch", *response.Output[0].Input)
	assert.NotContains(t, recorder.Body.String(), "cursor_external_tool_calls")
}

func TestCursorStreamResponseConvertsCustomToolCallToResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"apply_patch": {Kind: dto.CustomType, Name: "apply_patch"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-responses-tool-stream")

	toolEnvelope := `{"cursor_external_tool_calls":[{"id":"call_patch","name":"apply_patch","input":"*** Begin Patch"}]}`
	responseHeader := make(http.Header)
	responseHeader.Set(cursorClientStreamHeader, "true")
	responseHeader.Set(cursorSkipRemoteUsageHeader, "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body:       io.NopCloser(strings.NewReader("event: assistant\ndata: {\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	assert.Contains(t, body, "event: response.created\n")
	assert.Contains(t, body, `"type":"custom_tool_call","id":"call_patch"`)
	assert.Contains(t, body, "event: response.custom_tool_call_input.delta\n")
	assert.Contains(t, body, `"delta":"*** Begin Patch"`)
	assert.Contains(t, body, "event: response.custom_tool_call_input.done\n")
	assert.Contains(t, body, `"input":"*** Begin Patch"`)
	assert.Contains(t, body, "event: response.completed\n")
	assert.NotContains(t, body, "cursor_external_tool_calls")
}

func TestCursorResponsesStreamKeepsNormalTextLive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(cursorExternalToolsContextKey, map[string]cursorExternalToolSpec{"apply_patch": {Kind: dto.CustomType, Name: "apply_patch"}})
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-responses-text")

	responseHeader := make(http.Header)
	responseHeader.Set(cursorClientStreamHeader, "true")
	responseHeader.Set(cursorSkipRemoteUsageHeader, "true")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     responseHeader,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: assistant",
			`data: {"text":"Hello "}`,
			"",
			"event: assistant",
			`data: {"text":"from Cursor"}`,
			"",
			"event: result",
			`data: {"runId":"run-1","status":"FINISHED","text":"Hello from Cursor"}`,
			"",
		}, "\n"))),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := recorder.Body.String()
	assert.Contains(t, body, `"type":"response.output_text.delta","delta":"Hello "`)
	assert.Contains(t, body, `"type":"response.output_text.delta","delta":"from Cursor"`)
	assert.Contains(t, body, "event: response.completed\n")
}

func TestCursorResponsesNamespaceFunctionRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("id", 1)
	common.SetContextKey(c, common.RequestIdKey, "req-cursor-responses-namespace")

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}, dto.OpenAIResponsesRequest{
		Model: "composer-2",
		Input: []byte(`[
			{"type":"additional_tools","role":"developer","tools":[
				{"type":"namespace","name":"collaboration","tools":[
					{"type":"function","name":"send_message","description":"Send a message","parameters":{"type":"object"}}
				]}
			]},
			{"role":"user","content":"Send a message"}
		]`),
	})
	require.NoError(t, err)
	specs := cursorExternalToolSpecs(c)
	assert.Equal(t, cursorExternalToolSpec{
		Kind:      "function",
		Name:      "send_message",
		Namespace: "collaboration",
	}, specs["collaboration__send_message"])

	toolEnvelope := `{"cursor_external_tool_calls":[{"id":"call_send","name":"collaboration__send_message","arguments":{"target":"worker"}}]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			cursorClientStreamHeader:    []string{"false"},
			cursorSkipRemoteUsageHeader: []string{"true"},
		},
		Body: io.NopCloser(strings.NewReader("event: assistant\ndata: {\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"" + strings.ReplaceAll(toolEnvelope, `"`, `\"`) + "\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Output, 1)
	assert.Equal(t, "function_call", response.Output[0].Type)
	assert.Equal(t, "send_message", response.Output[0].Name)
	assert.Equal(t, "collaboration", response.Output[0].Namespace)
	assert.Equal(t, "call_send", response.Output[0].CallId)
}

func TestCursorAdaptorRejectsUnsupportedGeminiEndpoint(t *testing.T) {
	adaptor := &Adaptor{}

	_, err := adaptor.ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
	require.ErrorContains(t, err, "Gemini endpoint not supported")
}
