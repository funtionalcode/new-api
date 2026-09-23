package relay

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type responsesWebsocketMessage struct {
	messageType int
	payload     []byte
	err         error
}

// 每条连接始终只有一个读取协程，轮次之间也继续处理 Ping、Pong 和关闭帧。
type responsesWebsocketPeer struct {
	conn     *websocket.Conn
	messages <-chan responsesWebsocketMessage
	stop     context.CancelFunc
	closeErr atomic.Pointer[websocket.CloseError]
}

func newResponsesWebsocketPeer(conn *websocket.Conn) *responsesWebsocketPeer {
	ctx, stop := context.WithCancel(context.Background())
	messages := make(chan responsesWebsocketMessage, 1)
	peer := &responsesWebsocketPeer{conn: conn, messages: messages, stop: stop}
	closeHandler := conn.CloseHandler()
	conn.SetCloseHandler(func(code int, text string) error {
		// 先保存关闭原因，下一轮写入失败时仍能将原始关闭码传给客户端。
		peer.closeErr.Store(&websocket.CloseError{Code: code, Text: text})
		return closeHandler(code, text)
	})

	go func() {
		defer close(messages)
		defer stop()
		for {
			messageType, payload, err := conn.ReadMessage()
			select {
			case messages <- responsesWebsocketMessage{messageType, payload, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			// WriteControl 可与数据帧写入并发；连接建立后先发送一次心跳。
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				_ = conn.Close()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	// 调用方停止心跳后仍可发送最终错误，随后由连接所有者关闭底层连接并解除读取阻塞。
	return peer
}

func (peer *responsesWebsocketPeer) ReadMessage() (int, []byte, error) {
	message, ok := <-peer.messages
	if !ok {
		return 0, nil, io.EOF
	}
	return message.messageType, message.payload, message.err
}

func (peer *responsesWebsocketPeer) WriteMessage(messageType int, payload []byte) error {
	if closeErr := peer.closeErr.Load(); closeErr != nil {
		return closeErr
	}
	err := peer.conn.WriteMessage(messageType, payload)
	if err != nil {
		if closeErr := peer.closeErr.Load(); closeErr != nil {
			return closeErr
		}
	}
	return err
}
