package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelStatusErrorDetailsGroupsSamplesByModelAndBucket(t *testing.T) {
	base := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC).Unix()
	samples := []model.ModelStatusErrorSample{
		{
			ModelName: "gpt-alpha",
			CreatedAt: base + 3660,
			Content:   "upstream overloaded",
			Other:     `{"status_code":503,"error_type":"server_error","error_code":"overloaded","admin_info":{"use_channel":[1]}}`,
		},
		{
			ModelName: "gpt-alpha",
			CreatedAt: base + 120,
			Content:   "rate limited",
			Other:     `{"status_code":429,"error_type":"rate_limit_error","error_code":"rate_limit"}`,
		},
		{
			ModelName: "gpt-beta",
			CreatedAt: base + 180,
			Content:   "bad request",
			Other:     `{"status_code":"400","error_type":"invalid_request_error"}`,
		},
		{
			ModelName: " ",
			CreatedAt: base + 240,
			Content:   "ignored",
		},
	}

	detailsByModel, detailsByBucket := modelStatusErrorDetails(samples)

	require.Len(t, detailsByModel["gpt-alpha"], 2)
	assert.Equal(t, "upstream overloaded", detailsByModel["gpt-alpha"][0].Message)
	assert.Equal(t, 503, detailsByModel["gpt-alpha"][0].StatusCode)
	assert.Equal(t, "server_error", detailsByModel["gpt-alpha"][0].ErrorType)
	assert.Equal(t, "overloaded", detailsByModel["gpt-alpha"][0].ErrorCode)
	assert.Equal(t, "rate limited", detailsByModel["gpt-alpha"][1].Message)

	require.Len(t, detailsByBucket["gpt-alpha"][base+3600], 1)
	assert.Equal(t, "upstream overloaded", detailsByBucket["gpt-alpha"][base+3600][0].Message)
	require.Len(t, detailsByBucket["gpt-alpha"][base], 1)
	assert.Equal(t, "rate limited", detailsByBucket["gpt-alpha"][base][0].Message)

	require.Len(t, detailsByModel["gpt-beta"], 1)
	assert.Equal(t, 400, detailsByModel["gpt-beta"][0].StatusCode)
	assert.NotContains(t, detailsByModel, " ")
}
