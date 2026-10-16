package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestJevDailyUsageSeparatesMainRequestsAndEvaluationCosts(t *testing.T) {
	useSeparatedUserConsumptionTestDB(t)
	const start int64 = 1780740000
	require.NoError(t, DB.Create(&[]CliproxyAuthFileBinding{
		{UserId: 11, Username: "alice", AuthIndex: "auth-a", AuthName: "a.json", Provider: "codex"},
		{UserId: 11, Username: "alice", AuthIndex: "idle", AuthName: "idle.json"},
		{UserId: 12, Username: "bob", AuthIndex: "other-user", AuthName: "b.json"},
	}).Error)
	logs := []Log{
		{UserId: 11, Username: "alice", TokenId: 1, TokenName: "auth-a", ChannelId: 7, RequestId: "used", ModelName: "gpt-main", PromptTokens: 100, CompletionTokens: 20, Quota: 200, UseTime: 3, CreatedAt: start,
			Other: `{"admin_info":{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"success","applied":true}]},"typesafe":[{"status":"skipped"}]}`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ChannelId: 21, RequestId: "child", ModelName: "TypeSafe/JEV-1.13.0", PromptTokens: 10, CompletionTokens: 2, Quota: 5, CreatedAt: start + 1,
			Other: `{"admin_info":{"typesafe":[{"stage":"adaptive_reasoning","parent_request_id":"used","parent_channel_id":7}]}}`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ChannelId: 7, RequestId: "cached", ModelName: "gpt-main", CompletionTokens: 30, Quota: 30, CreatedAt: start + 2,
			Other: `{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"success","applied":true,"source":"cache"}]}`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "gpt-main", PromptTokens: 50, CompletionTokens: 10, Quota: 60, CreatedAt: start + 3},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "gpt-main", Quota: 7, CreatedAt: start + 4,
			Other: `{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"error","applied":false}]}`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "gpt-main", Quota: 8, CreatedAt: start + 5, Other: `{bad`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "jev-latest", PromptTokens: 3, Quota: 9, CreatedAt: start + 6},
		{UserId: 12, TokenId: 1, TokenName: "other-user", ModelName: "jev-latest", Quota: 11, CreatedAt: start + 7,
			Other: `{"admin_info":{"typesafe":[{"parent_request_id":"used","parent_channel_id":7}]}}`},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "gpt-main", Quota: 1000, CreatedAt: start + 86400},
		{UserId: 11, TokenId: 1, TokenName: "auth-a", ModelName: "gpt-main", Quota: 1000, CreatedAt: start - 1},
	}
	for i := range logs {
		logs[i].Type = LogTypeConsume
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)
	report, err := GetJevDailyUsage(context.Background(), start, start+86400)
	require.NoError(t, err)
	assert.Equal(t, 8, report.LogCount)
	byStatus := make(map[string]JevDailyUsageRow)
	for _, row := range report.Rows {
		byStatus[fmt.Sprintf("%d:%s", row.UserID, row.JevStatus)] = row
	}
	used := byStatus["11:used"]
	assert.True(t, used.Bound)
	assert.Equal(t, "a.json", used.AuthName)
	assert.Equal(t, "codex", used.Provider)
	assert.Equal(t, JevDailyUsage{RequestCount: 2, PromptTokens: 100, CompletionTokens: 50, Quota: 230, UseTimeSeconds: 3, CachedDecisions: 1, EvaluationRequests: 1, EvaluationPrompt: 10, EvaluationCompletion: 2, EvaluationQuota: 5}, used.JevDailyUsage)
	assert.Equal(t, int64(60), byStatus["11:not_used"].Quota)
	assert.Equal(t, int64(7), byStatus["11:not_applied"].Quota)
	assert.Equal(t, int64(8), byStatus["11:unknown"].Quota)
	assert.Equal(t, int64(9), byStatus["11:evaluation_only"].EvaluationQuota)
	assert.Equal(t, int64(11), byStatus["12:evaluation_only"].EvaluationQuota)
	assert.Equal(t, "idle.json", byStatus["11:no_activity"].AuthName)
	assert.Zero(t, byStatus["11:no_activity"].RequestCount)
	var mainRequests, evaluations, quota int64
	for _, row := range report.Rows {
		mainRequests += row.RequestCount
		evaluations += row.EvaluationRequests
		quota += row.Quota + row.EvaluationQuota
	}
	assert.Equal(t, int64(5), mainRequests)
	assert.Equal(t, int64(3), evaluations)
	assert.Equal(t, int64(330), quota, "同日主请求与评估的已记录成本只能各统计一次")
}

func TestJevDailyUsageClassifiesSkippedAndSuccessfulStages(t *testing.T) {
	for _, tc := range []struct {
		name, other, want string
		cached            bool
	}{
		{"历史成功评估", `{"typesafe":[{"stage":"before","model":"jev-latest","status":"success"}]}`, "used", false},
		{"旧记录缺少应用状态", `{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"success"}]}`, "unknown", false},
		{"有评估但未应用", `{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"success","applied":false}]}`, "not_applied", false},
		{"跳过", `{"typesafe":[{"stage":"adaptive_reasoning","status":"skipped"}]}`, "not_applied", false},
		{"非 Jev 评估", `{"typesafe":[{"stage":"before","model":"other-evaluator","status":"success"}]}`, "not_used", false},
		{"其他自适应评估模型", `{"typesafe":[{"stage":"adaptive_reasoning","model":"other-evaluator","status":"success","applied":true}]}`, "not_used", false},
		{"重试复用", `{"typesafe":[{"stage":"adaptive_reasoning","model":"jev-latest","status":"success","applied":true,"source":"request_retry"}]}`, "used", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var meta jevUsageMetadata
			require.NoError(t, common.UnmarshalJsonStr(tc.other, &meta))
			status, cached, _ := jevUsageCohort(meta)
			assert.Equal(t, tc.want, status)
			assert.Equal(t, tc.cached, cached)
		})
	}
	for _, tc := range []struct{ start, end int64 }{{0, 1}, {100, 99}, {100, 100 + 27*3600}} {
		_, err := GetJevDailyUsage(context.Background(), tc.start, tc.end)
		require.Error(t, err)
	}
}

func TestJevDailyUsageChildEvidenceDoesNotMisclassifyOtherRequests(t *testing.T) {
	useSeparatedUserConsumptionTestDB(t)
	const start int64 = 1780740000
	require.NoError(t, LOG_DB.Create(&[]Log{
		{UserId: 1, TokenId: 1, TokenName: "auth", ChannelId: 7, ModelName: "main", Type: LogTypeConsume, RequestId: "missing-status", Quota: 100, CreatedAt: start},
		{UserId: 1, TokenId: 1, TokenName: "auth", ChannelId: 7, ModelName: "main", Type: LogTypeConsume, RequestId: "no-evaluation", Quota: 200, CreatedAt: start + 1},
		{UserId: 1, TokenId: 1, TokenName: "auth", ChannelId: 21, ModelName: "jev-latest", Type: LogTypeConsume, Quota: 3, CreatedAt: start + 2,
			Other: `{"admin_info":{"typesafe":[{"stage":"adaptive_reasoning","parent_request_id":"missing-status","parent_channel_id":7}]}}`},
		{UserId: 1, TokenId: 1, TokenName: "auth", ChannelId: 21, ModelName: "jev-latest", Type: LogTypeConsume, Quota: 4, CreatedAt: start + 3,
			Other: `{"admin_info":{"typesafe":[{"stage":"after","parent_request_id":"missing-status","parent_channel_id":7}]}}`},
	}).Error)
	report, err := GetJevDailyUsage(context.Background(), start, start+86400)
	require.NoError(t, err)
	require.Len(t, report.Rows, 2)
	for _, row := range report.Rows {
		assert.Equal(t, int64(1), row.RequestCount)
		if row.JevStatus == "unknown" {
			assert.Equal(t, int64(100), row.Quota)
			assert.Equal(t, int64(7), row.EvaluationQuota)
			assert.Equal(t, int64(2), row.EvaluationRequests)
		} else {
			assert.Equal(t, "not_used", row.JevStatus)
			assert.Equal(t, int64(200), row.Quota)
			assert.Zero(t, row.EvaluationRequests)
		}
	}
}

func useSeparatedUserConsumptionTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalLogDatabaseType := common.LogDatabaseType()
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.SetLogDatabaseType(originalLogDatabaseType)
	})

	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(&User{}, &Channel{}, &CliproxyAuthFileBinding{}))

	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&Log{}))

	DB = mainDB
	LOG_DB = logDB
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
}

func TestGetUserTokenUsageSummaryWithSeparatedLogDBEnrichesMainDBFields(t *testing.T) {
	useSeparatedUserConsumptionTestDB(t)

	require.NoError(t, DB.Create(&User{
		Id:       11,
		Username: "alice",
		Remark:   "重点用户",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:   7,
		Name: "codex-channel",
	}).Error)
	require.NoError(t, DB.Create(&CliproxyAuthFileBinding{
		UserId:    11,
		Username:  "alice",
		AuthIndex: "codex-main",
		AuthName:  "Codex 主号",
		Enabled:   true,
	}).Error)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			UserId:           11,
			Username:         "alice",
			TokenId:          101,
			TokenName:        "codex-main",
			ChannelId:        7,
			ModelName:        "gpt-5",
			Type:             LogTypeConsume,
			PromptTokens:     120,
			CompletionTokens: 30,
			Quota:            500,
			CreatedAt:        now - 10,
		},
		{
			UserId:           11,
			Username:         "alice",
			TokenId:          101,
			TokenName:        "codex-main",
			ChannelId:        7,
			ModelName:        "gpt-5",
			Type:             LogTypeConsume,
			PromptTokens:     40,
			CompletionTokens: 10,
			Quota:            100,
			CreatedAt:        now,
		},
		{
			UserId:           11,
			Username:         "alice",
			TokenId:          101,
			TokenName:        "codex-main",
			ChannelId:        7,
			ModelName:        "gpt-5",
			Type:             LogTypeConsume,
			PromptTokens:     1000,
			CompletionTokens: 1000,
			Quota:            1000,
			CreatedAt:        now - 3600,
		},
	}).Error)

	summaries, total, err := GetUserTokenUsageSummary(UserTokenUsageQuery{
		StartTimestamp: now - 60,
		EndTimestamp:   now + 60,
		AuthIndex:      "codex-main",
	}, 0, 10)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, summaries, 1)
	require.Equal(t, 11, summaries[0].UserId)
	require.Equal(t, "alice", summaries[0].Username)
	require.Equal(t, "重点用户", summaries[0].Remark)
	require.Equal(t, "codex-main", summaries[0].TokenName)
	require.Equal(t, "codex-main", summaries[0].AuthIndex)
	require.Equal(t, "Codex 主号", summaries[0].AuthName)
	require.Equal(t, "codex-channel", summaries[0].ChannelName)
	require.Equal(t, int64(2), summaries[0].RequestCount)
	require.Equal(t, int64(160), summaries[0].PromptTokens)
	require.Equal(t, int64(40), summaries[0].CompletionTokens)
	require.Equal(t, int64(200), summaries[0].TotalTokens)
	require.Equal(t, int64(600), summaries[0].Quota)
}

func TestGetUserTokenUsageByDayWithSeparatedLogDBAvoidsMainDBJoins(t *testing.T) {
	useSeparatedUserConsumptionTestDB(t)

	require.NoError(t, DB.Create(&User{
		Id:       12,
		Username: "bob",
		Remark:   "测试备注",
	}).Error)
	require.NoError(t, DB.Create(&Channel{
		Id:   8,
		Name: "deepseek-channel",
	}).Error)

	createdAt := int64(1780743000)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           12,
		Username:         "bob",
		TokenId:          202,
		TokenName:        "deepseek-main",
		ChannelId:        8,
		ModelName:        "deepseek-chat",
		Type:             LogTypeConsume,
		PromptTokens:     10,
		CompletionTokens: 20,
		Quota:            30,
		CreatedAt:        createdAt,
	}).Error)

	summaries, total, err := GetUserTokenUsageByDay(UserTokenUsageQuery{
		StartTimestamp: createdAt - 1,
		EndTimestamp:   createdAt + 1,
	}, 0, 10)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, summaries, 1)
	require.Equal(t, 12, summaries[0].UserId)
	require.Equal(t, "测试备注", summaries[0].Remark)
	require.Equal(t, "deepseek-channel", summaries[0].ChannelName)
	require.Equal(t, int64(30), summaries[0].TotalTokens)
}
