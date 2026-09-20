package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeTypeSafeResultsOmitsChildAndAdminFields(t *testing.T) {
	results := []map[string]any{
		{
			"stage":      "before",
			"status":     "success",
			"channel_id": 21,
			"model":      "jev-latest",
			"request_id": "child-before",
			"truncated":  true,
			"answers":    map[string]any{"category": map[string]any{"type": "choice", "choice": "coding"}},
			"usage":      map[string]any{"input_tokens": 12},
		},
		{
			"stage":             "before",
			"parent_request_id": "parent-1",
			"answers":           map[string]any{"leaked": true},
		},
		{
			"stage":  "after",
			"status": "error",
			"reason": "evaluation_failed",
		},
	}

	summary := SanitizeTypeSafeResults(results)
	require.Len(t, summary, 2)
	assert.Equal(t, "before", summary[0]["stage"])
	assert.Equal(t, "success", summary[0]["status"])
	assert.Equal(t, true, summary[0]["truncated"])
	assert.Equal(t, map[string]any{"category": map[string]any{"type": "choice", "choice": "coding"}}, summary[0]["answers"])
	assert.NotContains(t, summary[0], "channel_id")
	assert.NotContains(t, summary[0], "model")
	assert.NotContains(t, summary[0], "request_id")
	assert.NotContains(t, summary[0], "usage")
	assert.Equal(t, "after", summary[1]["stage"])
	assert.Equal(t, "evaluation_failed", summary[1]["reason"])
}

func TestFormatUserLogsPromotesTypeSafeSummaryWithoutExchange(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.01,
		"admin_info": map[string]interface{}{
			"typesafe": []any{
				map[string]any{
					"stage":      "before",
					"status":     "success",
					"channel_id": 9,
					"answers":    map[string]any{"check": map[string]any{"type": "noul", "noul": 0.9}},
				},
				map[string]any{
					"stage":             "after",
					"parent_request_id": "parent-1",
				},
			},
			"typesafe_exchange": map[string]any{
				"request": map[string]any{"body": "private input"},
			},
		},
	})
	logs := []*Log{{Other: other, ChannelName: "secret-channel"}}

	formatUserLogs(logs, 0)

	assert.Empty(t, logs[0].ChannelName)
	assert.NotContains(t, logs[0].Other, "private input")
	assert.NotContains(t, logs[0].Other, "typesafe_exchange")
	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "admin_info")
	assert.Equal(t, 0.01, parsed["model_price"])
	summary, ok := parsed["typesafe"].([]any)
	require.True(t, ok)
	require.Len(t, summary, 1)
	item, ok := summary[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "before", item["stage"])
	assert.NotContains(t, item, "channel_id")
	answers, ok := item["answers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "noul", "noul": 0.9}, answers["check"])
}

func TestBuildTypeSafeUserEvaluationGroupsBeforeAndAfter(t *testing.T) {
	log := &Log{
		CreatedAt: 1710000000,
		ModelName: "gpt-4.1",
		TokenName: "canary",
		RequestId: "req-1",
		Group:     "default",
		Other: common.MapToJsonStr(map[string]interface{}{
			"typesafe": []any{
				map[string]any{
					"stage":   "before",
					"status":  "success",
					"answers": map[string]any{"category": map[string]any{"type": "choice", "choice": "coding"}},
				},
				map[string]any{
					"stage":  "after",
					"status": "skipped",
					"reason": "no_text_output",
				},
			},
		}),
	}

	evaluation := buildTypeSafeUserEvaluation(log)
	require.NotNil(t, evaluation)
	assert.Equal(t, "gpt-4.1", evaluation.ModelName)
	require.NotNil(t, evaluation.Before)
	assert.Equal(t, "success", evaluation.Before.Status)
	assert.Equal(t, "coding", evaluation.Before.Answers["category"].(map[string]any)["choice"])
	require.NotNil(t, evaluation.After)
	assert.Equal(t, "skipped", evaluation.After.Status)
	assert.Equal(t, "no_text_output", evaluation.After.Reason)
}

func TestBuildTypeSafeUserEvaluationSkipsChildLogs(t *testing.T) {
	log := &Log{
		Other: common.MapToJsonStr(map[string]interface{}{
			"typesafe": []any{
				map[string]any{"stage": "before", "parent_request_id": "parent-1"},
			},
		}),
	}
	assert.Nil(t, buildTypeSafeUserEvaluation(log))
}
