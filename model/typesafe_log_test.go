package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestJevLogVisibilityFiltersBeforePaginationAndPreservesRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previousDB, previousLogDB := DB, LOG_DB
	previousSetting := operation_setting.GetGeneralSetting().ShowJevLogs
	previousOptions := common.OptionMap
	DB, LOG_DB = db, db
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.OptionMap = previousOptions
		operation_setting.GetGeneralSetting().ShowJevLogs = previousSetting
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&Log{}, &User{}, &Channel{}, &Option{}))
	models := []string{"gpt-5", "jev", "jev-latest", "TypeSafe/JEV-1.13.0", "vendor/team/jev", "other-jev-model", "jevish", ""}
	for _, name := range models {
		require.NoError(t, db.Create(&Log{UserId: 7, ModelName: name, ChannelId: 21, Type: LogTypeConsume, Other: "{}"}).Error)
	}
	require.NoError(t, db.Create(&Log{UserId: 8, ModelName: "other-user", ChannelId: 21, Type: LogTypeConsume, Other: "{}"}).Error)
	require.NoError(t, db.Create(&Log{UserId: 7, ModelName: "hidden-channel", ChannelId: 22, Type: LogTypeConsume, Other: "{}"}).Error)

	for _, scope := range []string{"all", "self"} {
		t.Run(scope, func(t *testing.T) {
			query := func(offset, limit int, modelName string) ([]*Log, int64, error) {
				if scope == "self" {
					return GetUserLogs(7, LogTypeUnknown, 0, 0, modelName, "", offset, limit, "", "", "", "", []int{21})
				}
				return GetAllLogs(LogTypeUnknown, 0, 0, modelName, "", "", offset, limit, 0, "", "", "", "", "", []int{21})
			}
			var expectedTotal int64 = 4
			if scope == "all" {
				expectedTotal++
			}
			require.NoError(t, UpdateOption("general_setting.show_jev_logs", "false"))
			var saved Option
			require.NoError(t, db.First(&saved, "key = ?", "general_setting.show_jev_logs").Error)
			assert.Equal(t, "false", saved.Value)
			var actual []string
			for offset := 0; offset < int(expectedTotal); offset++ {
				logs, total, err := query(offset, 1, "")
				require.NoError(t, err)
				assert.Equal(t, expectedTotal, total)
				require.Len(t, logs, 1)
				actual = append(actual, logs[0].ModelName)
			}
			expected := []string{"gpt-5", "other-jev-model", "jevish", ""}
			if scope == "all" {
				expected = append(expected, "other-user")
			}
			assert.ElementsMatch(t, expected, actual)
			logs, total, err := query(0, 20, "jev-latest")
			require.NoError(t, err)
			assert.Empty(t, logs)
			assert.Zero(t, total)

			require.NoError(t, UpdateOption("general_setting.show_jev_logs", "true"))
			logs, total, err = query(0, 20, "")
			require.NoError(t, err)
			assert.Equal(t, expectedTotal+4, total)
			assert.Len(t, logs, int(total))
		})
	}
	var storedCount int64
	require.NoError(t, db.Model(&Log{}).Count(&storedCount).Error)
	assert.Equal(t, int64(10), storedCount)
}

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
	assert.Equal(t, "jev-latest", summary[0]["model"])
	assert.NotContains(t, summary[0], "request_id")
	assert.NotContains(t, summary[0], "usage")
	assert.Equal(t, "after", summary[1]["stage"])
	assert.Equal(t, "evaluation_failed", summary[1]["reason"])
}

func TestFormatUserLogsPromotesTypeSafeSummaryWithoutExchange(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.01,
		"typesafe":    []any{map[string]any{"stage": "before", "status": "success"}},
		"admin_info": map[string]interface{}{
			"typesafe": []any{
				map[string]any{
					"stage":      "before",
					"status":     "success",
					"channel_id": 9,
					"model":      "jev-latest",
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
	assert.Equal(t, "jev-latest", item["model"])
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
					"model":   "jev-latest",
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
	assert.Equal(t, "jev-latest", evaluation.Before.Model)
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
