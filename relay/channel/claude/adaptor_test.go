package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertClaudeRequestTreatsZeroMaxTokensAsUnset(t *testing.T) {
	zero := uint(0)
	req := &dto.ClaudeRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: &zero,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-sonnet-4-5",
		},
	}

	out, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	require.NoError(t, err)
	converted, ok := out.(*dto.ClaudeRequest)
	require.True(t, ok)
	require.NotNil(t, converted.MaxTokens)
	assert.Equal(t, uint(model_setting.GetClaudeSettings().GetDefaultMaxTokens(req.Model)), *converted.MaxTokens)
}

func TestConvertClaudeRequestZeroMaxTokensStillRaisesThinkingBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	zero := uint(0)
	original := &dto.ClaudeRequest{
		Model:     "claude-3-7-sonnet-thinking",
		MaxTokens: &zero,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		Request:         original,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet-thinking",
		},
	}
	outbound, err := common.DeepCopy(original)
	require.NoError(t, err)
	require.NoError(t, helper.ModelMappedHelper(c, info, outbound))
	require.NoError(t, helper.ApplyReasoningModelSuffix(info, outbound))

	out, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, outbound)
	require.NoError(t, err)
	converted, ok := out.(*dto.ClaudeRequest)
	require.True(t, ok)
	assert.Equal(t, "claude-3-7-sonnet", converted.Model)
	require.NotNil(t, converted.Thinking)
	require.NotNil(t, converted.MaxTokens)
	assert.Greater(t, *converted.MaxTokens, uint(1024))
}

func TestConvertClaudeRequestDoesNotOverwriteTrimmedUpstreamModelName(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "claude-3-7-sonnet-thinking",
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet",
		},
	}

	_, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	require.NoError(t, err)
	assert.Equal(t, "claude-3-7-sonnet", info.UpstreamModelName)
}

func TestConvertClaudeRequestRemovesEmptySystemContent(t *testing.T) {
	req := &dto.ClaudeRequest{
		Model: "claude-fable-5-1",
		System: []any{
			map[string]any{"type": "text", "text": "  "},
			map[string]any{"type": "text", "text": "Keep this system instruction."},
		},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "first"},
			{Role: "system", Content: []any{}},
			{Role: "developer", Content: []any{map[string]any{"type": "text", "text": ""}}},
			{Role: "assistant", Content: "answer"},
			{Role: "system", Content: []any{map[string]any{"type": "text", "text": "keep"}}},
			{Role: "user", Content: "next"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fable-5-1"},
	}

	out, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, req)
	require.NoError(t, err)
	converted, ok := out.(*dto.ClaudeRequest)
	require.True(t, ok)

	system, ok := converted.System.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, system, 2)
	assert.Equal(t, "Keep this system instruction.", system[0].GetText())
	assert.Equal(t, "keep", system[1].GetText())
	require.Len(t, converted.Messages, 3)
	assert.Equal(t, []string{"user", "assistant", "user"}, []string{
		converted.Messages[0].Role,
		converted.Messages[1].Role,
		converted.Messages[2].Role,
	})

	emptyReq := &dto.ClaudeRequest{
		Model:    "claude-fable-5-1",
		System:   []any{},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
	}
	_, err = (&Adaptor{}).ConvertClaudeRequest(nil, info, emptyReq)
	require.NoError(t, err)
	assert.Nil(t, emptyReq.System)
	encoded, err := common.Marshal(emptyReq)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"system"`)
}
