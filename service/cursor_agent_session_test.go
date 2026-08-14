package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCursorAgentSessionTestContext(t *testing.T, path string, body string, userID int, modelName string) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
	ctx.Set("id", userID)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	storage, err := common.CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	ctx.Set(common.KeyBodyStorage, storage)
	t.Cleanup(func() { _ = storage.Close() })
	PrepareCursorAgentSession(ctx, modelName)
	return ctx
}

func TestPrepareCursorAgentSessionRestoresClaudeCodeConversation(t *testing.T) {
	first := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"model":"claude-sonnet-5",
		"metadata":{"user_id":"user-7-session-claude-1"}
	}`, 7, "claude-sonnet-5")

	assert.Equal(t, "true", first.GetHeader(constant.CursorPersistentHeader))
	assert.Empty(t, first.GetHeader(constant.CursorAgentIDHeader))
	require.NotEmpty(t, common.GetContextKeyString(first, constant.ContextKeyCursorAgentSession))
	require.NoError(t, SaveCursorAgentSession(first, CursorAgentSession{
		AgentID:   "bc-00000000-0000-0000-0000-000000000001",
		Signature: "v2.claude-signature",
		ChannelID: 17,
		KeyIndex:  2,
	}))

	second := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"model":"claude-sonnet-5",
		"metadata":{"user_id":"user-7-session-claude-1"}
	}`, 7, "claude-sonnet-5")

	assert.Equal(t, "bc-00000000-0000-0000-0000-000000000001", second.GetHeader(constant.CursorAgentIDHeader))
	assert.Equal(t, "v2.claude-signature", second.GetHeader(constant.CursorAgentSignatureHeader))
	assert.Equal(t, "17", second.GetHeader(constant.CursorAgentChannelIDHeader))
	assert.Equal(t, "2", second.GetHeader(constant.CursorAgentKeyIndexHeader))
}

func TestPrepareCursorAgentSessionRestoresCodexConversation(t *testing.T) {
	first := newCursorAgentSessionTestContext(t, "/v1/responses", `{
		"model":"gpt-5.6-sol",
		"prompt_cache_key":"019ffebf-c59e-7780-9033-65c19a254938"
	}`, 8, "gpt-5.6-sol")
	require.NoError(t, SaveCursorAgentSession(first, CursorAgentSession{
		AgentID:   "bc-00000000-0000-0000-0000-000000000002",
		Signature: "v2.codex-signature",
		ChannelID: 18,
		KeyIndex:  0,
	}))

	second := newCursorAgentSessionTestContext(t, "/v1/responses", `{
		"model":"gpt-5.6-sol",
		"prompt_cache_key":"019ffebf-c59e-7780-9033-65c19a254938"
	}`, 8, "gpt-5.6-sol")

	assert.Equal(t, "bc-00000000-0000-0000-0000-000000000002", second.GetHeader(constant.CursorAgentIDHeader))
	assert.Equal(t, "18", second.GetHeader(constant.CursorAgentChannelIDHeader))
	assert.Equal(t, "0", second.GetHeader(constant.CursorAgentKeyIndexHeader))
}

func TestCursorAgentSessionIsIsolatedByUserModelAndClaudeSubagent(t *testing.T) {
	first := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"metadata":{"user_id":"shared-session"}
	}`, 9, "claude-sonnet-5")
	require.NoError(t, SaveCursorAgentSession(first, CursorAgentSession{
		AgentID:   "bc-00000000-0000-0000-0000-000000000003",
		Signature: "v2.isolated-signature",
		ChannelID: 19,
		KeyIndex:  1,
	}))

	otherUser := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"metadata":{"user_id":"shared-session"}
	}`, 10, "claude-sonnet-5")
	assert.Empty(t, otherUser.GetHeader(constant.CursorAgentIDHeader))

	otherModel := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"metadata":{"user_id":"shared-session"}
	}`, 9, "claude-opus-5")
	assert.Empty(t, otherModel.GetHeader(constant.CursorAgentIDHeader))

	subagent := newCursorAgentSessionTestContext(t, "/v1/messages", `{
		"metadata":{"user_id":"shared-session"}
	}`, 9, "claude-sonnet-5")
	subagent.Request.Header.Set("X-Claude-Code-Agent-ID", "agent-1")
	subagent.Request.Header.Del(constant.CursorAgentIDHeader)
	subagent.Request.Header.Del(constant.CursorAgentSignatureHeader)
	subagent.Request.Header.Del(constant.CursorAgentChannelIDHeader)
	subagent.Request.Header.Del(constant.CursorAgentKeyIndexHeader)
	// Re-run preparation after adding the header to model the incoming request.
	PrepareCursorAgentSession(subagent, "claude-sonnet-5")
	assert.Empty(t, subagent.GetHeader(constant.CursorAgentIDHeader))
}

func TestDeleteCursorAgentSessionPreventsFurtherReuse(t *testing.T) {
	first := newCursorAgentSessionTestContext(t, "/v1/responses", `{
		"prompt_cache_key":"codex-cleanup-session"
	}`, 11, "gpt-5.6-sol")
	require.NoError(t, SaveCursorAgentSession(first, CursorAgentSession{
		AgentID:   "bc-00000000-0000-0000-0000-000000000004",
		Signature: "v2.cleanup-signature",
		ChannelID: 20,
		KeyIndex:  0,
	}))
	require.NoError(t, DeleteCursorAgentSession(first))

	afterCleanup := newCursorAgentSessionTestContext(t, "/v1/responses", `{
		"prompt_cache_key":"codex-cleanup-session"
	}`, 11, "gpt-5.6-sol")
	assert.Empty(t, afterCleanup.GetHeader(constant.CursorAgentIDHeader))
}

func TestPrepareCursorAgentSessionUsesCodexSessionHeaderFallback(t *testing.T) {
	ctx := newCursorAgentSessionTestContext(t, "/v1/responses", `{}`, 12, "gpt-5.6-sol")
	ctx.Request.Header.Set("Session-Id", "codex-header-session")
	PrepareCursorAgentSession(ctx, "gpt-5.6-sol")

	assert.Equal(t, "true", ctx.GetHeader(constant.CursorPersistentHeader))
	assert.NotEmpty(t, common.GetContextKeyString(ctx, constant.ContextKeyCursorAgentSession))
}

func TestCursorAgentSessionIsIsolatedByReasoningEffort(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		model       string
		firstBody   string
		changedBody string
		userID      int
	}{
		{
			name:        "Claude Code output config",
			path:        "/v1/messages",
			model:       "claude-opus-5",
			firstBody:   `{"metadata":{"user_id":"effort-session"},"output_config":{"effort":"high"}}`,
			changedBody: `{"metadata":{"user_id":"effort-session"},"output_config":{"effort":"max"}}`,
			userID:      31,
		},
		{
			name:        "Codex reasoning config",
			path:        "/v1/responses",
			model:       "gpt-5.6-sol",
			firstBody:   `{"prompt_cache_key":"effort-session","reasoning":{"effort":"high"}}`,
			changedBody: `{"prompt_cache_key":"effort-session","reasoning":{"effort":"xhigh"}}`,
			userID:      32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := newCursorAgentSessionTestContext(t, tt.path, tt.firstBody, tt.userID, tt.model)
			require.NoError(t, SaveCursorAgentSession(first, CursorAgentSession{
				AgentID:   "bc-00000000-0000-0000-0000-000000000099",
				Signature: "v2.reasoning-signature",
				ChannelID: 33,
				KeyIndex:  0,
			}))

			sameEffort := newCursorAgentSessionTestContext(t, tt.path, tt.firstBody, tt.userID, tt.model)
			assert.Equal(t, "bc-00000000-0000-0000-0000-000000000099", sameEffort.GetHeader(constant.CursorAgentIDHeader))

			changedEffort := newCursorAgentSessionTestContext(t, tt.path, tt.changedBody, tt.userID, tt.model)
			assert.Empty(t, changedEffort.GetHeader(constant.CursorAgentIDHeader))
		})
	}
}
