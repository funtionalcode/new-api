package service

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/tidwall/gjson"
)

const (
	cursorAgentSessionCacheNamespace = "new-api:cursor_agent_session:v1"
	cursorAgentSessionCacheCapacity  = 100_000
)

var (
	cursorAgentSessionCacheOnce sync.Once
	cursorAgentSessionCache     *cachex.HybridCache[CursorAgentSession]
)

// CursorAgentSession binds a client conversation to the Cursor Agent and
// channel key that created it. Entries have no time-based TTL; the normal
// deletion path is the client's explicit cleanup command.
type CursorAgentSession struct {
	AgentID   string `json:"agent_id"`
	Signature string `json:"signature"`
	ChannelID int    `json:"channel_id"`
	KeyIndex  int    `json:"key_index"`
}

func getCursorAgentSessionCache() *cachex.HybridCache[CursorAgentSession] {
	cursorAgentSessionCacheOnce.Do(func() {
		cursorAgentSessionCache = cachex.NewHybridCache[CursorAgentSession](cachex.HybridCacheConfig[CursorAgentSession]{
			Namespace: cachex.Namespace(cursorAgentSessionCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[CursorAgentSession]{},
			Memory: func() *hot.HotCache[string, CursorAgentSession] {
				return hot.NewHotCache[string, CursorAgentSession](hot.LRU, cursorAgentSessionCacheCapacity).Build()
			},
		})
	})
	return cursorAgentSessionCache
}

// PrepareCursorAgentSession restores server-managed Cursor Agent headers for
// Claude Code and Codex requests. These clients preserve their own stable
// conversation identifiers but do not round-trip new-api response headers.
func PrepareCursorAgentSession(c *gin.Context, modelName string) {
	if c == nil || c.Request == nil || c.Request.Method != http.MethodPost {
		return
	}
	userID := c.GetInt("id")
	modelName = strings.TrimSpace(modelName)
	if userID <= 0 || modelName == "" {
		return
	}

	clientKind, clientSessionID, reasoningEffort := cursorAgentClientSession(c)
	if clientSessionID == "" {
		return
	}
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	cacheKey := strings.Join([]string{
		strconv.Itoa(userID),
		clientKind,
		common.Sha1([]byte(modelName)),
		common.Sha1([]byte(usingGroup)),
		common.Sha1([]byte(clientSessionID)),
		common.Sha1([]byte(reasoningEffort)),
	}, ":")
	common.SetContextKey(c, constant.ContextKeyCursorAgentSession, cacheKey)
	c.Request.Header.Set(constant.CursorPersistentHeader, "true")

	// Explicit client-managed persistence remains authoritative (for example,
	// the playground already round-trips these headers itself).
	if strings.TrimSpace(c.GetHeader(constant.CursorAgentIDHeader)) != "" {
		return
	}

	session, found, err := getCursorAgentSessionCache().Get(cacheKey)
	if err != nil {
		common.SysError(fmt.Sprintf("cursor agent session cache get failed: %v", err))
		return
	}
	if !found || strings.TrimSpace(session.AgentID) == "" || strings.TrimSpace(session.Signature) == "" || session.ChannelID <= 0 || session.KeyIndex < 0 {
		return
	}

	c.Request.Header.Set(constant.CursorAgentIDHeader, session.AgentID)
	c.Request.Header.Set(constant.CursorAgentSignatureHeader, session.Signature)
	c.Request.Header.Set(constant.CursorAgentChannelIDHeader, strconv.Itoa(session.ChannelID))
	c.Request.Header.Set(constant.CursorAgentKeyIndexHeader, strconv.Itoa(session.KeyIndex))
}

func cursorAgentClientSession(c *gin.Context) (string, string, string) {
	path := c.Request.URL.Path
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return "", "", ""
	}
	body, err := storage.Bytes()
	if err != nil || len(body) == 0 {
		return "", "", ""
	}

	switch path {
	case "/v1/messages":
		sessionID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())
		if sessionID == "" {
			sessionID = strings.TrimSpace(c.GetHeader("X-Claude-Remote-Session-ID"))
		}
		if sessionID == "" {
			return "", "", ""
		}
		if agentID := strings.TrimSpace(c.GetHeader("X-Claude-Code-Agent-ID")); agentID != "" {
			sessionID += ":agent:" + agentID
		}
		reasoningEffort := strings.TrimSpace(gjson.GetBytes(body, "output_config.effort").String())
		return "claude-code", sessionID, reasoningEffort
	case "/v1/responses":
		sessionID := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
		if sessionID == "" {
			for _, header := range []string{"Session-Id", "Session_id", "Thread-Id", "Thread_id"} {
				sessionID = strings.TrimSpace(c.GetHeader(header))
				if sessionID != "" {
					break
				}
			}
		}
		if sessionID == "" {
			return "", "", ""
		}
		reasoningEffort := strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())
		return "codex", sessionID, reasoningEffort
	default:
		return "", "", ""
	}
}

// SaveCursorAgentSession records a newly created or validated Agent for the
// stable client conversation prepared earlier in the request.
func SaveCursorAgentSession(c *gin.Context, session CursorAgentSession) error {
	if c == nil {
		return nil
	}
	cacheKey := common.GetContextKeyString(c, constant.ContextKeyCursorAgentSession)
	if cacheKey == "" {
		return nil
	}
	session.AgentID = strings.TrimSpace(session.AgentID)
	session.Signature = strings.TrimSpace(session.Signature)
	if session.AgentID == "" || session.Signature == "" || session.ChannelID <= 0 || session.KeyIndex < 0 {
		return fmt.Errorf("cursor agent session is incomplete")
	}
	return getCursorAgentSessionCache().SetWithTTL(cacheKey, session, 0)
}

// DeleteCursorAgentSession removes the server-side binding after the remote
// Agent has been deleted, or when its signature is no longer usable.
func DeleteCursorAgentSession(c *gin.Context) error {
	if c == nil {
		return nil
	}
	cacheKey := common.GetContextKeyString(c, constant.ContextKeyCursorAgentSession)
	if cacheKey == "" {
		return nil
	}
	_, err := getCursorAgentSessionCache().DeleteMany([]string{cacheKey})
	return err
}
