package reasoning

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderGeminiMapsEffortToSupportedLevel(t *testing.T) {
	tests := []struct {
		model  string
		effort Effort
		want   Effort
	}{
		{"gemini-3.8-flash", EffortMinimal, EffortLow},
		{"gemini-3.8-flash-high", EffortMinimal, EffortLow},
		{"gemini-3.7-flash", EffortMinimal, EffortLow},
		{"gemini-3.8-flash", EffortLow, EffortLow},
		{"gemini-3.8-flash", EffortMedium, EffortMedium},
		{"gemini-3.8-flash", EffortHigh, EffortHigh},
		{"gemini-3.8-flash", EffortXHigh, EffortHigh},
		{"gemini-3.6-flash", EffortMinimal, EffortMinimal},
		{"gemini-3.5-flash", EffortMinimal, EffortMinimal},
		{"gemini-3.5-flash-lite", EffortMinimal, EffortMinimal},
		{"gemini-3-flash-preview", EffortMinimal, EffortMinimal},
		{"gemini-3.1-pro-preview", EffortMinimal, EffortLow},
	}
	for _, tt := range tests {
		t.Run(tt.model+"/"+string(tt.effort), func(t *testing.T) {
			rendered, err := RenderGemini(tt.model, Intent{Effort: tt.effort, Source: SourceExplicit}, nil, 0)
			require.NoError(t, err)
			require.NotNil(t, rendered.Config)
			assert.Equal(t, string(tt.want), rendered.Config.ThinkingLevel)
			assert.Nil(t, rendered.Config.ThinkingBudget)
			assert.Equal(t, tt.want, rendered.EffectiveEffort)
		})
	}
}

func TestValidateGeminiThinkingConfigRejectsUnsupportedNativeLevel(t *testing.T) {
	for _, model := range []string{"gemini-3.7-flash", "gemini-3.8-flash"} {
		t.Run(model, func(t *testing.T) {
			config := &dto.GeminiThinkingConfig{ThinkingLevel: "minimal"}
			_, err := ValidateGeminiThinkingConfig(model, config)
			require.ErrorContains(t, err, "is not supported")
			assert.Equal(t, "minimal", config.ThinkingLevel)

			config.ThinkingLevel = "low"
			effort, err := ValidateGeminiThinkingConfig(model, config)
			require.NoError(t, err)
			assert.Equal(t, EffortLow, effort)
		})
	}
}

func TestResolveGeminiFlashDefaultEffort(t *testing.T) {
	for _, model := range []string{"gemini-3.7-flash", "gemini-3.8-flash"} {
		t.Run(model, func(t *testing.T) {
			intent := ResolveGeminiDefault(model, Intent{})
			assert.Equal(t, ModeEnabled, intent.Mode)
			assert.Equal(t, EffortMedium, intent.Effort)

			intent = ResolveGeminiEnabledDefault(model, Intent{Mode: ModeEnabled, Source: SourceSuffix}, nil)
			rendered, err := RenderGemini(model, intent, nil, 0)
			require.NoError(t, err)
			require.NotNil(t, rendered.Config)
			assert.Equal(t, "medium", rendered.Config.ThinkingLevel)
			assert.Equal(t, EffortMedium, rendered.EffectiveEffort)
		})
	}
}

func TestNormalizeGeminiThinkingConfigMapsNativeMinimalForNewFlashModels(t *testing.T) {
	for _, tt := range []struct{ model, want string }{
		{"gemini-3.7-flash", "low"},
		{"gemini-3.8-flash-high", "low"},
		{"gemini-3.6-flash", "minimal"},
	} {
		t.Run(tt.model, func(t *testing.T) {
			includeThoughts := false
			config := &dto.GeminiThinkingConfig{ThinkingLevel: "minimal", IncludeThoughts: &includeThoughts}
			effort, err := NormalizeGeminiThinkingConfig(tt.model, config)
			require.NoError(t, err)
			assert.Equal(t, tt.want, config.ThinkingLevel)
			assert.Equal(t, Effort(tt.want), effort)
			require.NotNil(t, config.IncludeThoughts)
			assert.False(t, *config.IncludeThoughts)
			assert.Nil(t, config.ThinkingBudget)
		})
	}
}

func TestNormalizeGeminiThinkingConfigPreservesInvalidRequestForRejection(t *testing.T) {
	budget := 0
	for _, config := range []*dto.GeminiThinkingConfig{
		{ThinkingLevel: "invalid"},
		{ThinkingLevel: "minimal", ThinkingBudget: &budget},
	} {
		original := *config
		_, err := NormalizeGeminiThinkingConfig("gemini-3.8-flash-high", config)
		require.Error(t, err)
		assert.Equal(t, original, *config)
	}
}
