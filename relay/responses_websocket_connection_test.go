package relay

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesWebsocketPeerKeepsControlFramesLiveWithPendingRequest(t *testing.T) {
	for _, gatewayIsServer := range []bool{true, false} {
		name := "upstream"
		if gatewayIsServer {
			name = "client"
		}
		t.Run(name, func(t *testing.T) {
			server, client := newResponsesWebsocketPair(t)
			gateway, remote := client, server
			if gatewayIsServer {
				gateway, remote = server, client
			}
			require.NoError(t, remote.SetReadDeadline(time.Now().Add(3*time.Second)))
			pings := make(chan struct{}, 1)
			pongs := make(chan string, 1)
			defaultPingHandler := remote.PingHandler()
			remote.SetPingHandler(func(payload string) error {
				pings <- struct{}{}
				return defaultPingHandler(payload)
			})
			remote.SetPongHandler(func(payload string) error {
				pongs <- payload
				return nil
			})
			peer := newResponsesWebsocketPeer(gateway)
			defer peer.stop()
			remoteMessages := make(chan responsesWebsocketMessage, 1)
			go func() {
				messageType, payload, err := remote.ReadMessage()
				remoteMessages <- responsesWebsocketMessage{messageType, payload, err}
			}()

			// 即使业务暂未取走下一轮请求，也必须响应控制帧。
			request := []byte(`{"type":"response.create","model":"test-model","input":[]}`)
			require.NoError(t, remote.WriteMessage(websocket.TextMessage, request))
			require.NoError(t, remote.WriteControl(websocket.PingMessage, []byte("idle-probe"), time.Now().Add(time.Second)))
			select {
			case pong := <-pongs:
				assert.Equal(t, "idle-probe", pong)
			case <-time.After(3 * time.Second):
				t.Fatal("等待下一轮请求时未响应 Ping")
			}
			select {
			case <-pings:
			case <-time.After(3 * time.Second):
				t.Fatal("连接未主动发送心跳")
			}

			messageType, payload, err := peer.ReadMessage()
			require.NoError(t, err)
			assert.Equal(t, websocket.TextMessage, messageType)
			assert.Equal(t, request, payload)

			// 停止心跳后，连接所有者仍可发送最终错误再关闭连接。
			peer.stop()
			response := []byte(`{"type":"error","error":{"message":"test"}}`)
			require.NoError(t, gateway.WriteMessage(websocket.TextMessage, response))
			message := <-remoteMessages
			require.NoError(t, message.err)
			assert.Equal(t, websocket.TextMessage, message.messageType)
			assert.Equal(t, response, message.payload)
		})
	}
}

func TestResponsesWebsocketPeerPreservesIdleCloseOnNextRequest(t *testing.T) {
	upstream, gateway := newResponsesWebsocketPair(t)
	require.NoError(t, gateway.SetReadDeadline(time.Now().Add(3*time.Second)))
	peer := newResponsesWebsocketPeer(gateway)
	defer peer.stop()
	require.NoError(t, upstream.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseMessageTooBig, "message too big"), time.Now().Add(time.Second)))
	_, _, err := peer.ReadMessage()
	require.Error(t, err)

	// 空闲时已收到关闭帧，下一轮不能丢失关闭码或把请求重放到其他连接。
	err = peer.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create"}`))
	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
	assert.Equal(t, websocket.CloseMessageTooBig, closeErr.Code)
	assert.Equal(t, "message too big", closeErr.Text)
}
