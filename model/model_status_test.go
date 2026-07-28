package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelStatusLogSummariesAggregatesConsumeAndErrorLogs(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	base := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			ModelName: "gpt-alpha", CreatedAt: base + 60, Type: LogTypeConsume,
			Quota: 100, PromptTokens: 10, CompletionTokens: 20, UseTime: 4,
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base + 120, Type: LogTypeConsume,
			Quota: 200, PromptTokens: 20, CompletionTokens: 30, UseTime: 6,
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base + 180, Type: LogTypeError,
			UseTime: 2,
		},
		{
			ModelName: "gpt-beta", CreatedAt: base + 240, Type: LogTypeError,
			UseTime: 3,
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base - 60, Type: LogTypeConsume,
			Quota: 999, PromptTokens: 99, CompletionTokens: 99, UseTime: 9,
		},
	}).Error)

	rows, err := GetModelStatusLogSummaries(base, base+3600, 10)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	byModel := map[string]ModelStatusLogSummary{}
	for _, row := range rows {
		byModel[row.ModelName] = row
	}

	alpha := byModel["gpt-alpha"]
	assert.Equal(t, int64(3), alpha.RequestCount)
	assert.Equal(t, int64(2), alpha.SuccessCount)
	assert.Equal(t, int64(1), alpha.ErrorCount)
	assert.Equal(t, int64(80), alpha.TokenCount)
	assert.Equal(t, int64(300), alpha.Quota)
	assert.InDelta(t, 5.0, alpha.AvgUseTime, 0.001)
	assert.Equal(t, int64(10), alpha.TotalUseTime)
	assert.Equal(t, int64(50), alpha.CompletionTokens)

	beta := byModel["gpt-beta"]
	assert.Equal(t, int64(1), beta.RequestCount)
	assert.Equal(t, int64(0), beta.SuccessCount)
	assert.Equal(t, int64(1), beta.ErrorCount)
	assert.Equal(t, int64(0), beta.TokenCount)
}

func TestGetModelStatusLogBucketsGroupsByHour(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	base := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{ModelName: "gpt-alpha", CreatedAt: base + 60, Type: LogTypeConsume},
		{ModelName: "gpt-alpha", CreatedAt: base + 120, Type: LogTypeError},
		{ModelName: "gpt-alpha", CreatedAt: base + 3600, Type: LogTypeConsume},
		{ModelName: "gpt-beta", CreatedAt: base + 60, Type: LogTypeConsume},
	}).Error)

	rows, err := GetModelStatusLogBuckets(base, base+7200, 3600, []string{"gpt-alpha"})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, base, rows[0].BucketStart)
	assert.Equal(t, int64(2), rows[0].RequestCount)
	assert.Equal(t, int64(1), rows[0].SuccessCount)
	assert.Equal(t, int64(1), rows[0].ErrorCount)
	assert.Equal(t, base+3600, rows[1].BucketStart)
	assert.Equal(t, int64(1), rows[1].RequestCount)
	assert.Equal(t, int64(1), rows[1].SuccessCount)
}

func TestGetModelStatusTodaySummaryUsesConsumeLogsOnly(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	base := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			ModelName: "gpt-alpha", CreatedAt: base + 60, Type: LogTypeConsume,
			Quota: 100, PromptTokens: 10, CompletionTokens: 20,
		},
		{
			ModelName: "gpt-beta", CreatedAt: base + 120, Type: LogTypeConsume,
			Quota: 200, PromptTokens: 30, CompletionTokens: 40,
		},
		{
			ModelName: "gpt-beta", CreatedAt: base + 180, Type: LogTypeError,
			Quota: 900, PromptTokens: 90, CompletionTokens: 90,
		},
	}).Error)

	summary, err := GetModelStatusTodaySummary(base, base+3600)

	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.RequestCount)
	assert.Equal(t, int64(100), summary.TokenCount)
	assert.Equal(t, int64(300), summary.Quota)
}

func TestGetModelStatusErrorSamplesReturnsRecentWindowErrors(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)

	base := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, LOG_DB.Create(&[]Log{
		{
			ModelName: "gpt-alpha", CreatedAt: base + 60, Type: LogTypeError,
			Content: "first error", Other: `{"status_code":500}`, RequestId: "req-first",
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base + 120, Type: LogTypeConsume,
			Content: "successful request",
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base + 180, Type: LogTypeError,
			Content: "latest error", Other: `{"status_code":429}`, RequestId: "req-latest",
		},
		{
			ModelName: "gpt-beta", CreatedAt: base + 240, Type: LogTypeError,
			Content: "other model error",
		},
		{
			ModelName: "gpt-alpha", CreatedAt: base - 60, Type: LogTypeError,
			Content: "old error",
		},
	}).Error)

	rows, err := GetModelStatusErrorSamples(base, base+3600, []string{"gpt-alpha"}, 10)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "latest error", rows[0].Content)
	assert.Equal(t, base+180, rows[0].CreatedAt)
	assert.Equal(t, "req-latest", rows[0].RequestId)
	assert.Equal(t, "first error", rows[1].Content)
	assert.Equal(t, `{"status_code":500}`, rows[1].Other)
	assert.Equal(t, "req-first", rows[1].RequestId)
}
