package reasoning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderClaudeConvertsNativeBudgetForAdaptiveOnlyModel(t *testing.T) {
	t.Parallel()

	budget := 4096
	render, err := RenderClaude("claude-fable-5-1", Intent{
		Mode:         ModeEnabled,
		BudgetTokens: &budget,
		Source:       SourceNative,
		BudgetSource: SourceNative,
	}, nil, 0.8)

	require.NoError(t, err)
	require.NotNil(t, render.Thinking)
	assert.Equal(t, "adaptive", render.Thinking.Type)
	assert.Nil(t, render.Thinking.BudgetTokens)
	assert.Equal(t, EffortMedium, render.OutputEffort)
	assert.Equal(t, EffortMedium, render.EffectiveEffort)
}

func TestRenderClaudePreservesNativeBudgetForManualModel(t *testing.T) {
	t.Parallel()

	budget := 4096
	maxTokens := uint(8192)
	render, err := RenderClaude("claude-3-7-sonnet", Intent{
		Mode:         ModeEnabled,
		BudgetTokens: &budget,
		Source:       SourceNative,
		BudgetSource: SourceNative,
	}, &maxTokens, 0.8)

	require.NoError(t, err)
	require.NotNil(t, render.Thinking)
	assert.Equal(t, "enabled", render.Thinking.Type)
	require.NotNil(t, render.Thinking.BudgetTokens)
	assert.Equal(t, budget, *render.Thinking.BudgetTokens)
}
