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

	require.Equal(t, true, other["ws"])
	require.Equal(t, "websocket", other["transport"])
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

			require.Equal(t, lifecycle, other["cursor_agent_lifecycle"])
		})
	}
}
