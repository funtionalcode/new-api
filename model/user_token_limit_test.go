package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetUserTokenPeriodUsageUsesCurrentDayWeekMonthWindows(t *testing.T) {
	truncateTables(t)

	loc := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 20, 15, 30, 0, 0, loc)
	userId := 42

	logs := []*Log{
		{UserId: userId, CreatedAt: time.Date(2026, time.August, 20, 1, 0, 0, 0, loc).Unix(), Type: LogTypeConsume, PromptTokens: 100, CompletionTokens: 20},
		{UserId: userId, CreatedAt: time.Date(2026, time.August, 18, 10, 0, 0, 0, loc).Unix(), Type: LogTypeConsume, PromptTokens: 30, CompletionTokens: 40},
		{UserId: userId, CreatedAt: time.Date(2026, time.August, 5, 9, 0, 0, 0, loc).Unix(), Type: LogTypeConsume, PromptTokens: 5, CompletionTokens: 6},
		{UserId: userId, CreatedAt: time.Date(2026, time.July, 31, 23, 59, 0, 0, loc).Unix(), Type: LogTypeConsume, PromptTokens: 1000, CompletionTokens: 1000},
		{UserId: userId, CreatedAt: time.Date(2026, time.August, 20, 2, 0, 0, 0, loc).Unix(), Type: LogTypeError, PromptTokens: 500, CompletionTokens: 500},
		{UserId: 99, CreatedAt: time.Date(2026, time.August, 20, 3, 0, 0, 0, loc).Unix(), Type: LogTypeConsume, PromptTokens: 700, CompletionTokens: 700},
	}
	require.NoError(t, LOG_DB.Create(&logs).Error)

	usage, err := GetUserTokenPeriodUsage(userId, now)
	require.NoError(t, err)

	require.Equal(t, int64(120), usage.Daily)
	require.Equal(t, int64(190), usage.Weekly)
	require.Equal(t, int64(201), usage.Monthly)
}

func TestCheckUserTokenLimitReportsReachedDailyLimit(t *testing.T) {
	truncateTables(t)

	loc := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 20, 15, 30, 0, 0, loc)
	user := &User{
		Username:          "limited",
		Password:          "password123",
		DailyTokenLimit:   100,
		WeeklyTokenLimit:  500,
		MonthlyTokenLimit: 1000,
	}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           user.Id,
		CreatedAt:        time.Date(2026, time.August, 20, 1, 0, 0, 0, loc).Unix(),
		Type:             LogTypeConsume,
		PromptTokens:     60,
		CompletionTokens: 40,
	}).Error)

	result, err := CheckUserTokenLimit(user.Id, now)
	require.NoError(t, err)

	require.True(t, result.Exceeded)
	require.Equal(t, TokenLimitPeriodDaily, result.Period)
	require.Equal(t, int64(100), result.Used)
	require.Equal(t, int64(100), result.Limit)
}
