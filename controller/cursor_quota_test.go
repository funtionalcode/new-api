package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCursorQuotaUsageParsesBillingResponses(t *testing.T) {
	periodBody := []byte(`{
		"billingCycleStart":"1786609998000",
		"billingCycleEnd":"1789288398000",
		"planUsage":{
			"totalSpend":5288,
			"limit":40000,
			"remaining":34712,
			"autoPercentUsed":0.0045,
			"apiPercentUsed":10.558,
			"totalPercentUsed":2.1152
		},
		"spendLimitUsage":{"individualLimit":15000,"individualRemaining":12500}
	}`)
	aggregatedBody := []byte(`{
		"aggregations":[
			{"modelIntent":"claude-fable-5-thinking-high","inputTokens":"100","outputTokens":"20","cacheWriteTokens":"3","cacheReadTokens":"4","totalCents":12.5,"tier":1},
			{"modelIntent":"cursor-grok-4.6-high-fast","inputTokens":"200","outputTokens":"30","cacheWriteTokens":"0","cacheReadTokens":"8","totalCents":0,"tier":2}
		],
		"totalInputTokens":"300",
		"totalOutputTokens":"50",
		"totalCacheWriteTokens":"3",
		"totalCacheReadTokens":"12",
		"totalCostCents":12.5
	}`)
	planBody := []byte(`{"planInfo":{"planName":"Ultra","includedAmountCents":40000,"price":"$200/mo"}}`)

	usage, err := extractCursorQuotaUsage(periodBody, aggregatedBody, planBody)
	require.NoError(t, err)

	assert.Equal(t, "Ultra", usage.PlanName)
	assert.Equal(t, int64(1_786_609_998), usage.BillingCycleStartAt)
	assert.Equal(t, int64(1_789_288_398), usage.BillingCycleEndAt)
	assert.InDelta(t, 5288, usage.PlanUsedCents, 0.000001)
	assert.InDelta(t, 40000, usage.PlanLimitCents, 0.000001)
	assert.InDelta(t, 34712, usage.PlanRemainingCents, 0.000001)
	assert.InDelta(t, 10.558, usage.PlanAPIPercent, 0.000001)
	assert.InDelta(t, 2.1152, usage.PlanTotalPercent, 0.000001)
	assert.InDelta(t, 2500, usage.OnDemandUsedCents, 0.000001)
	assert.Equal(t, int64(300), usage.TotalInputTokens)
	assert.Equal(t, int64(50), usage.TotalOutputTokens)
	assert.Equal(t, int64(3), usage.TotalCacheWriteTokens)
	assert.Equal(t, int64(12), usage.TotalCacheReadTokens)
	assert.InDelta(t, 12.5, usage.TotalCostCents, 0.000001)
	require.Len(t, usage.ModelUsages, 2)
	assert.Equal(t, int64(127), usage.ModelUsages[0].TotalTokens)
	assert.Equal(t, 1, usage.ModelUsages[0].Tier)
	assert.Equal(t, int64(238), usage.ModelUsages[1].TotalTokens)
	assert.Equal(t, 2, usage.ModelUsages[1].Tier)
}

func TestParseCursorQuotaTimestampAcceptsISOAndEpochValues(t *testing.T) {
	isoTime, err := time.Parse(time.RFC3339Nano, "2026-08-13T00:00:00.000Z")
	require.NoError(t, err)

	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{name: "ISO timestamp", value: "2026-08-13T00:00:00.000Z", expected: isoTime.Unix()},
		{name: "epoch seconds", value: "1786609998", expected: 1_786_609_998},
		{name: "epoch milliseconds", value: "1786609998000", expected: 1_786_609_998},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := parseCursorQuotaTimestamp(test.value)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestUpdateCursorAggregatedUsageRequestBodyUsesCurrentBillingCycle(t *testing.T) {
	body, err := updateCursorAggregatedUsageRequestBody(
		`{"teamId":42,"startDate":123,"extra":"kept"}`,
		1_786_579_200,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"teamId":42,"startDate":1786579200000,"extra":"kept"}`, body)
}

func TestValidateCursorQuotaEndpointRejectsUnexpectedDestination(t *testing.T) {
	const currentPeriodPath = "/api/dashboard/get-current-period-usage"

	require.NoError(t, validateCursorQuotaEndpoint(
		"https://cursor.com/api/dashboard/get-current-period-usage",
		currentPeriodPath,
	))
	assert.Error(t, validateCursorQuotaEndpoint(
		"https://example.com/api/dashboard/get-current-period-usage",
		currentPeriodPath,
	))
	assert.Error(t, validateCursorQuotaEndpoint(
		"https://cursor.com/api/dashboard/get-plan-info",
		currentPeriodPath,
	))
	assert.Error(t, validateCursorQuotaEndpoint(
		"http://cursor.com/api/dashboard/get-current-period-usage",
		currentPeriodPath,
	))
}

func TestBuildCursorQuotaCurlRequestParsesChromeCopyAsCurl(t *testing.T) {
	rawCurl := `curl 'https://cursor.com/api/dashboard/get-current-period-usage' \
  -H 'content-type: application/json' \
  -H 'cookie: WorkosCursorSessionToken=session-value' \
  --data-raw '{}'`

	requestConfig, err := buildCursorQuotaCurlRequest(rawCurl)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, requestConfig.Method)
	assert.Equal(t, "https://cursor.com/api/dashboard/get-current-period-usage", requestConfig.URL)
	assert.Equal(t, "WorkosCursorSessionToken=session-value", requestConfig.Headers["Cookie"])
	assert.Equal(t, "{}", requestConfig.Body)
}
