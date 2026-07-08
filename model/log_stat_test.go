package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSumUsedQuotaReturnsAverageUseTimeForConfiguredWindow(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	now := time.Now().Unix()
	logs := []*Log{
		{
			Username: "alice", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 120, Type: LogTypeConsume, Quota: 100,
			PromptTokens: 10, CompletionTokens: 20, UseTime: 8,
			ChannelId: 3, Group: "vip", Ip: "192.0.2.1",
		},
		{
			Username: "alice", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 30, Type: LogTypeConsume, Quota: 200,
			PromptTokens: 20, CompletionTokens: 30, UseTime: 2,
			ChannelId: 3, Group: "vip", Ip: "192.0.2.1",
		},
		{
			Username: "alice", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 20, Type: LogTypeConsume, Quota: 300,
			PromptTokens: 30, CompletionTokens: 40, UseTime: 4,
			ChannelId: 3, Group: "vip", Ip: "192.0.2.1",
		},
		{
			Username: "alice", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 10, Type: LogTypeConsume, Quota: 400,
			PromptTokens: 40, CompletionTokens: 50, UseTime: 0,
			ChannelId: 3, Group: "vip", Ip: "192.0.2.1",
		},
		{
			Username: "alice", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 15, Type: LogTypeError, UseTime: 100,
			ChannelId: 3, Group: "vip", Ip: "192.0.2.1",
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	stat, err := SumUsedQuota(
		LogTypeUnknown,
		now-180,
		now,
		"gpt-test",
		"alice",
		"tok",
		3,
		"",
		"vip",
		"192.0.2.1",
		now-60,
		now,
		"",
		"",
	)
	require.NoError(t, err)

	require.Equal(t, 1000, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 80, stat.Tpm)
	require.InDelta(t, 3.0, stat.AvgUseTime, 0.001)
	require.Equal(t, int64(2), stat.AvgUseTimeCount)
}

func TestSumUsedQuotaAppliesRequestFiltersToAllStats(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	now := time.Now().Unix()
	logs := []*Log{
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 30, Type: LogTypeConsume, Quota: 100,
			PromptTokens: 10, CompletionTokens: 20, UseTime: 4,
			RequestId: "req-match", UpstreamRequestId: "up-match",
		},
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 20, Type: LogTypeConsume, Quota: 900,
			PromptTokens: 90, CompletionTokens: 90, UseTime: 30,
			RequestId: "req-other", UpstreamRequestId: "up-other",
		},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	stat, err := SumUsedQuota(
		LogTypeUnknown,
		now-60,
		now,
		"gpt-test",
		"root",
		"tok",
		0,
		"",
		"",
		"",
		now-60,
		now,
		"req-match",
		"up-match",
	)
	require.NoError(t, err)

	require.Equal(t, 100, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 30, stat.Tpm)
	require.InDelta(t, 4.0, stat.AvgUseTime, 0.001)
	require.Equal(t, int64(1), stat.AvgUseTimeCount)
}

func TestSumUsedQuotaAppliesLogTypeFilterToAverageUseTime(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 30, Type: LogTypeConsume, Quota: 100,
			PromptTokens: 10, CompletionTokens: 20, UseTime: 4,
		},
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 20, Type: LogTypeError, UseTime: 12,
		},
	}).Error)

	stat, err := SumUsedQuota(
		LogTypeError,
		now-60,
		now,
		"gpt-test",
		"root",
		"tok",
		0,
		"",
		"",
		"",
		now-60,
		now,
		"",
		"",
	)
	require.NoError(t, err)

	require.InDelta(t, 12.0, stat.AvgUseTime, 0.001)
	require.Equal(t, int64(1), stat.AvgUseTimeCount)
}

func TestLogQueriesFilterByChannelName(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, DB.Create(&[]Channel{
		{Id: 31, Name: "codex-usa", Status: 1},
		{Id: 32, Name: "deepseek-eu", Status: 1},
	}).Error)

	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 30, Type: LogTypeConsume, Quota: 100,
			PromptTokens: 10, CompletionTokens: 20, UseTime: 4,
			ChannelId: 31,
		},
		{
			Username: "root", TokenName: "tok", ModelName: "gpt-test",
			CreatedAt: now - 20, Type: LogTypeConsume, Quota: 900,
			PromptTokens: 90, CompletionTokens: 90, UseTime: 30,
			ChannelId: 32,
		},
	}).Error)

	logs, total, err := GetAllLogs(
		LogTypeConsume,
		now-60,
		now,
		"",
		"",
		"",
		0,
		10,
		0,
		"codex",
		"",
		"",
		"",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, 31, logs[0].ChannelId)
	require.Equal(t, "codex-usa", logs[0].ChannelName)

	stat, err := SumUsedQuota(
		LogTypeUnknown,
		now-60,
		now,
		"gpt-test",
		"root",
		"tok",
		0,
		"codex",
		"",
		"",
		now-60,
		now,
		"",
		"",
	)
	require.NoError(t, err)
	require.Equal(t, 100, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
	require.Equal(t, 30, stat.Tpm)
	require.InDelta(t, 4.0, stat.AvgUseTime, 0.001)
	require.Equal(t, int64(1), stat.AvgUseTimeCount)
}
