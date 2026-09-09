package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appcommon "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newResponsesWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConnCh := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnCh <- conn
		<-release
		_ = conn.Close()
	}))

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	serverConn := <-serverConnCh
	t.Cleanup(func() {
		_ = clientConn.Close()
		close(release)
		server.Close()
	})
	return serverConn, clientConn
}

func TestNormalizeResponsesWebsocketUpstreamPayloadRemovesXAITransportFields(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"grok-4.5","input":[],"stream":true,"background":true}`)
	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, appcommon.Unmarshal(payload, request))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		IsWebsocket:    true,
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           appconstant.APITypeXai,
			ChannelType:       appconstant.ChannelTypeXai,
			ChannelBaseUrl:    "https://api.x.ai",
			ApiKey:            "upstream-secret",
			UpstreamModelName: "grok-4.5",
		},
	}
	adaptor := GetAdaptor(info.ApiType)
	require.NotNil(t, adaptor)
	adaptor.Init(info)

	upstreamPayload, httpFallback, newAPIError := normalizeResponsesWebsocketUpstreamPayload(c, adaptor, info, payload, request)

	require.Nil(t, newAPIError)
	assert.False(t, httpFallback)
	assert.False(t, gjson.GetBytes(upstreamPayload, "stream").Exists())
	assert.False(t, gjson.GetBytes(upstreamPayload, "background").Exists())
	assert.Equal(t, "grok-4.5", gjson.GetBytes(upstreamPayload, "model").String())
}

func TestNormalizeResponsesWebsocketUpstreamPayloadUsesCursorHTTPFallback(t *testing.T) {
	payload := []byte(`{"type":"response.create","stream_id":"cursor-lane","model":"composer-2","input":"Hello from Codex"}`)
	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, appcommon.Unmarshal(payload, request))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		IsWebsocket:    true,
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           appconstant.APITypeCursor,
			ChannelType:       appconstant.ChannelTypeCursor,
			ChannelBaseUrl:    "https://api.cursor.com",
			ApiKey:            "cursor-secret",
			UpstreamModelName: "composer-2",
		},
	}
	adaptor := GetAdaptor(info.ApiType)
	require.NotNil(t, adaptor)
	adaptor.Init(info)

	upstreamPayload, httpFallback, newAPIError := normalizeResponsesWebsocketUpstreamPayload(c, adaptor, info, payload, request)

	require.Nil(t, newAPIError)
	assert.True(t, httpFallback)
	assert.Equal(t, "composer-2", gjson.GetBytes(upstreamPayload, "model.id").String())
	assert.Contains(t, gjson.GetBytes(upstreamPayload, "prompt.text").String(), "Hello from Codex")
	assert.False(t, gjson.GetBytes(upstreamPayload, "stream_id").Exists())
}

func TestCursorResponsesWebsocketHTTPFallbackStreamsJSONFrames(t *testing.T) {
	const (
		agentID = "bc-00000000-0000-0000-0000-000000000001"
		runID   = "run-1"
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
			_, _ = w.Write([]byte(`{"items":[{"id":"composer-2"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"agent":{"id":"` + agentID + `"},"run":{"id":"` + runID + `"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/stream"):
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: assistant\ndata: {\"text\":\"Hello from Cursor\"}\n\nevent: result\ndata: {\"runId\":\"run-1\",\"status\":\"FINISHED\",\"text\":\"Hello from Cursor\"}\n\nevent: done\ndata: {}\n\n"))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/usage"):
			_, _ = w.Write([]byte(`{"runs":[{"id":"run-1","usage":{"inputTokens":10,"outputTokens":5,"cacheWriteTokens":0,"cacheReadTokens":0,"totalTokens":15}}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/agents/"+agentID:
			_, _ = w.Write([]byte(`{"id":"` + agentID + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	serverWs, clientWs := newResponsesWebsocketPair(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	appcommon.SetContextKey(c, appcommon.RequestIdKey, "cursor-websocket-fallback")
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		IsWebsocket:    true,
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           appconstant.APITypeCursor,
			ChannelType:       appconstant.ChannelTypeCursor,
			ChannelBaseUrl:    upstream.URL,
			ApiKey:            "cursor-secret",
			UpstreamModelName: "composer-2",
		},
	}
	turn := &responsesWebsocketTurn{
		info:            info,
		upstreamPayload: []byte(`{"prompt":{"text":"Hello"},"model":{"id":"composer-2"}}`),
		httpFallback:    true,
		streamID:        "cursor-lane",
	}

	usage, newAPIError := forwardResponsesWebsocketHTTPFallback(c, serverWs, turn)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)

	require.NoError(t, clientWs.SetReadDeadline(time.Now().Add(2*time.Second)))
	seenDelta := false
	seenCompleted := false
	for !seenCompleted {
		_, message, err := clientWs.ReadMessage()
		require.NoError(t, err)
		assert.True(t, gjson.ValidBytes(message), "WebSocket frame must be raw JSON, not SSE")
		assert.Equal(t, "cursor-lane", gjson.GetBytes(message, "stream_id").String())
		switch gjson.GetBytes(message, "type").String() {
		case "response.output_text.delta":
			seenDelta = true
		case "response.completed":
			seenCompleted = true
		}
	}
	assert.True(t, seenDelta)
}

func TestCursorResponsesWebsocketFallbackPrewarmReturnsChainableResponse(t *testing.T) {
	serverWs, clientWs := newResponsesWebsocketPair(t)
	turn := &responsesWebsocketTurn{
		info: &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "composer-2"},
		},
		prewarm:      true,
		httpFallback: true,
		streamID:     "prewarm-lane",
	}

	require.NoError(t, completeResponsesWebsocketFallbackPrewarm(serverWs, turn))
	require.NoError(t, clientWs.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, created, err := clientWs.ReadMessage()
	require.NoError(t, err)
	_, completed, err := clientWs.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "response.created", gjson.GetBytes(created, "type").String())
	assert.Equal(t, "in_progress", gjson.GetBytes(created, "response.status").String())
	assert.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
	assert.Equal(t, "completed", gjson.GetBytes(completed, "response.status").String())
	assert.Equal(t, gjson.GetBytes(created, "response.id").String(), gjson.GetBytes(completed, "response.id").String())
	assert.Equal(t, "prewarm-lane", gjson.GetBytes(completed, "stream_id").String())
}
