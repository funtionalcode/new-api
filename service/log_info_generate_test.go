package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
