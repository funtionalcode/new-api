package newapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLPreservesClaudeCountTokensPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeClaudeCountTokens,
		RequestURLPath: "/v1/messages/count_tokens?beta=true",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://downstream.example",
			ChannelType:    constant.ChannelTypeNewAPI,
		},
	}

	got, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://downstream.example/v1/messages/count_tokens?beta=true", got)
}

func TestDoRequestUsesWebsocketForResponses(t *testing.T) {
	type requestSnapshot struct {
		path          string
		authorization string
		openAIBeta    string
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
			openAIBeta:    r.Header.Get("OpenAI-Beta"),
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
			ChannelType:    constant.ChannelTypeNewAPI,
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
	assert.Equal(t, relaycommon.ResponsesWebsocketBetaHeaderValue, request.openAIBeta)
	assert.Equal(t, "application/json", request.contentType)
}

func TestDoRequestAllowsNilHTTPBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: server.URL,
			ChannelType:    constant.ChannelTypeNewAPI,
			ApiKey:         "upstream-secret",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	result, err := adaptor.DoRequest(c, info, nil)

	require.NoError(t, err)
	resp, ok := result.(*http.Response)
	require.True(t, ok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
