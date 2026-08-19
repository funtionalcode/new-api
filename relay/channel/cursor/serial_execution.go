package cursor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

type cursorAgentSerialGate struct {
	slot chan struct{}
	refs int
}

type cursorAgentSerialGatePool struct {
	mutex sync.Mutex
	gates map[string]*cursorAgentSerialGate
}

var cursorAgentSerialGates = cursorAgentSerialGatePool{
	gates: make(map[string]*cursorAgentSerialGate),
}

func (p *cursorAgentSerialGatePool) acquire(ctx context.Context, key string) (func(), error) {
	p.mutex.Lock()
	gate := p.gates[key]
	if gate == nil {
		gate = &cursorAgentSerialGate{slot: make(chan struct{}, 1)}
		p.gates[key] = gate
	}
	gate.refs++
	p.mutex.Unlock()

	select {
	case gate.slot <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-gate.slot
				p.mutex.Lock()
				gate.refs--
				if gate.refs == 0 {
					delete(p.gates, key)
				}
				p.mutex.Unlock()
			})
		}, nil
	case <-ctx.Done():
		p.mutex.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(p.gates, key)
		}
		p.mutex.Unlock()
		return nil, ctx.Err()
	}
}

func acquireCursorAgentSerialExecution(c *gin.Context, info *relaycommon.RelayInfo) (func(), error) {
	if c == nil || c.Request == nil || info == nil || info.ChannelMeta == nil ||
		!info.ChannelSetting.CursorAgentSerialExecution || !c.GetBool(cursorPersistentContextKey) {
		return nil, nil
	}

	agentID := strings.TrimSpace(c.GetString(cursorAgentIDContextKey))
	if agentID == "" {
		return nil, nil
	}
	identity := common.GetContextKeyString(c, constant.ContextKeyCursorAgentSession)
	if identity == "" {
		identity = agentID
	}
	key := common.Sha1([]byte(identity))
	return cursorAgentSerialGates.acquire(c.Request.Context(), key)
}

type cursorAgentSerialResponseBody struct {
	io.ReadCloser
	release func()
	once    sync.Once
}

func (b *cursorAgentSerialResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

func transferCursorAgentSerialRelease(response *http.Response, release func()) {
	if response == nil || response.Body == nil || release == nil {
		return
	}
	response.Body = &cursorAgentSerialResponseBody{
		ReadCloser: response.Body,
		release:    release,
	}
}
