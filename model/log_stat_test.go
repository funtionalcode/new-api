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
		"vip",
		"192.0.2.1",
		now-60,
		now,
	)
	require.NoError(t, err)

	require.Equal(t, 1000, stat.Quota)
	require.InDelta(t, 3.0, stat.AvgUseTime, 0.001)
	require.Equal(t, int64(2), stat.AvgUseTimeCount)
}
