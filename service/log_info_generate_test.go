package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoMarksWebsocketTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)

	startTime := time.Unix(1000, 0)
	relayInfo := &relaycommon.RelayInfo{
		StartTime:         startTime,
		FirstResponseTime: startTime.Add(250 * time.Millisecond),
		IsWebsocket:       true,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, -1)

	require.Equal(t, true, other.Snapshot()["ws"])
	require.Equal(t, "websocket", other.Snapshot()["transport"])
}

func TestGenerateTextOtherInfoIncludesCursorAgentLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, lifecycle := range []string{constant.CursorAgentLifecycleCreate, constant.CursorAgentLifecycleDelete} {
		t.Run(lifecycle, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			common.SetContextKey(ctx, constant.ContextKeyCursorAgentLifecycle, lifecycle)

			startTime := time.Unix(1000, 0)
			relayInfo := &relaycommon.RelayInfo{
				StartTime:         startTime,
				FirstResponseTime: startTime,
				ChannelMeta:       &relaycommon.ChannelMeta{},
			}

			other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, -1)

			require.Equal(t, lifecycle, other.Snapshot()["cursor_agent_lifecycle"])
		})
	}
}

func TestGenerateTextOtherInfoIncludesSanitizedTypeSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	startTime := time.Unix(1000, 0)
	relayInfo := &relaycommon.RelayInfo{
		StartTime:         startTime,
		FirstResponseTime: startTime,
		ChannelMeta:       &relaycommon.ChannelMeta{},
		TypeSafeResults: []map[string]any{
			{
				"stage":      "before",
				"status":     "success",
				"channel_id": 21,
				"answers":    map[string]any{"check": map[string]any{"type": "noul", "noul": 0.9}},
			},
			{
				"stage":             "after",
				"parent_request_id": "parent-1",
			},
		},
		TypeSafeExchange: &relaycommon.TypeSafeExchange{
			Request: &relaycommon.TypeSafeLogBody{Body: "private input"},
		},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, -1)

	summary, ok := other.Snapshot()["typesafe"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, summary, 1)
	assert.Equal(t, "before", summary[0]["stage"])
	assert.NotContains(t, summary[0], "channel_id")
	assert.Equal(t, map[string]any{"check": map[string]any{"type": "noul", "noul": 0.9}}, summary[0]["answers"])

	admin, ok := other.Snapshot()["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, admin, "typesafe_exchange")
	assert.Equal(t, relayInfo.TypeSafeResults, admin["typesafe"])
}

func TestGenerateTextOtherInfoKeepsResponsesTerminalOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		event      string
		status     string
		outcome    string
		reason     string
		completion int
	}{
		{"output limit", `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":100,"output_tokens":128000,"total_tokens":128100}}}`, "error", "incomplete", "max_output_tokens", 128000},
		{"content filter", `{"type":"response.done","response":{"status":"incomplete","incomplete_details":{"reason":"content_filter"},"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}}`, "error", "incomplete", "content_filter", 5},
		{"cancelled", `{"type":"response.cancelled","response":{"status":"cancelled","usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}}`, "error", "cancelled", "", 5},
		{"failed", `{"type":"response.failed","response":{"status":"failed","usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}}`, "error", "failed", "", 5},
		{"completed", `{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}}`, "ok", "completed", "", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			start := time.Unix(1000, 0)
			info := &relaycommon.RelayInfo{
				StartTime: start, FirstResponseTime: start, IsStream: true,
				ChannelMeta: &relaycommon.ChannelMeta{}, StreamStatus: relaycommon.NewStreamStatus(),
			}
			var event dto.ResponsesStreamResponse
			require.NoError(t, common.UnmarshalJsonStr(tc.event, &event))
			accumulator := NewResponsesUsageAccumulator(info)
			accumulator.Observe(&event, []byte(tc.event))
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)

			usage := accumulator.Finish()
			other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, -1)
			stream, ok := other.Snapshot()["stream_status"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.status, stream["status"])
			assert.Equal(t, "done", stream["end_reason"])
			assert.Equal(t, tc.outcome, stream["response_status"])
			if tc.reason == "" {
				assert.NotContains(t, stream, "incomplete_reason")
			} else {
				assert.Equal(t, tc.reason, stream["incomplete_reason"])
			}
			assert.Equal(t, tc.status == "ok", RequestPolicy(ctx).Successful)
			assert.Equal(t, 100, usage.PromptTokens)
			assert.Equal(t, tc.completion, usage.CompletionTokens)
			assert.Equal(t, 100+tc.completion, usage.TotalTokens)
		})
	}
}
