package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
