package controller

import (
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
)

func TestBuildGLMQuotaUsageRequestParsesCurlAndRewritesWindow(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	rawCurl := `curl 'https://bigmodel.cn/api/monitor/usage/model-usage?startTime=2026-06-27+00:00:00&endTime=2026-07-03+23:59:59&type=3&refer__1090=abc' \
  -H 'accept: application/json, text/plain, */*' \
  -H 'authorization: token-value' \
  -H 'bigmodel-project: proj_1' \
  -b 'session=abc; token=def'`

	requestConfig, err := buildGLMQuotaUsageRequest(rawCurl, now)
	if err != nil {
		t.Fatalf("buildGLMQuotaUsageRequest returned error: %v", err)
	}
	if requestConfig.Method != "GET" {
		t.Fatalf("method = %s, want GET", requestConfig.Method)
	}
	if requestConfig.Headers["authorization"] != "token-value" {
		t.Fatalf("authorization header = %q", requestConfig.Headers["authorization"])
	}
	if requestConfig.Headers["Cookie"] != "session=abc; token=def" {
		t.Fatalf("cookie header = %q", requestConfig.Headers["Cookie"])
	}
	parsedURL, err := url.Parse(requestConfig.URL)
	if err != nil {
		t.Fatalf("normalized URL is invalid: %v", err)
	}
	query := parsedURL.Query()
	if query.Get("startTime") != "2026-06-27 00:00:00" {
		t.Fatalf("startTime = %q", query.Get("startTime"))
	}
	if query.Get("endTime") != "2026-07-03 23:59:59" {
		t.Fatalf("endTime = %q", query.Get("endTime"))
	}
	if query.Get("type") != "3" || query.Get("refer__1090") != "abc" {
		t.Fatalf("query was not preserved: %s", parsedURL.RawQuery)
	}
}

func TestApplyGLMQuotaPlanSpecDefaultsStandard(t *testing.T) {
	update := model.GLMQuotaBindingUpdate{PlanType: "标准版"}

	applyGLMQuotaPlanSpecDefaults(&update)

	if update.PlanType != "标准版" {
		t.Fatalf("PlanType = %q, want 标准版", update.PlanType)
	}
	if update.FiveHourLimitTokens != 60000000 {
		t.Fatalf("FiveHourLimitTokens = %d, want 60000000", update.FiveHourLimitTokens)
	}
	if update.WeeklyLimitTokens != 300000000 {
		t.Fatalf("WeeklyLimitTokens = %d, want 300000000", update.WeeklyLimitTokens)
	}
}

func TestExtractGLMQuotaUsageCalculatesFiveHourAndWeeklyUsage(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	body := []byte(`{
		"code": 200,
		"msg": "操作成功",
		"success": true,
		"data": {
			"x_time": [
				"2026-07-03 07:00",
				"2026-07-03 08:00",
				"2026-07-03 09:00",
				"2026-07-03 10:00",
				"2026-07-03 11:00",
				"2026-07-03 12:00"
			],
			"tokensUsage": [100, 1, 2, 3, 4, 5],
			"totalUsage": {
				"totalModelCallCount": 6,
				"totalTokensUsage": 33717,
				"modelSummaryList": [
					{"modelName": "GLM-5.2", "totalTokens": 33717, "sortOrder": 1}
				]
			}
		}
	}`)

	usage, err := extractGLMQuotaUsage(body, now)
	if err != nil {
		t.Fatalf("extractGLMQuotaUsage returned error: %v", err)
	}
	if usage.FiveHourUsedTokens != 15 {
		t.Fatalf("FiveHourUsedTokens = %d, want 15", usage.FiveHourUsedTokens)
	}
	if usage.WeeklyUsedTokens != 33717 {
		t.Fatalf("WeeklyUsedTokens = %d, want 33717", usage.WeeklyUsedTokens)
	}
	if usage.ModelCallCount != 6 {
		t.Fatalf("ModelCallCount = %d, want 6", usage.ModelCallCount)
	}
	if len(usage.ModelSummary) != 1 || usage.ModelSummary[0].ModelName != "GLM-5.2" {
		t.Fatalf("ModelSummary = %#v", usage.ModelSummary)
	}
}

func TestExtractGLMQuotaUsageFallsBackToModelDataSummary(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 30, 0, 0, time.Local)
	body := []byte(`{
		"code": 200,
		"success": true,
		"data": {
			"tokensUsage": [1, 2, 3, 4, 5, 6],
			"totalUsage": {"totalTokensUsage": 0},
			"modelDataList": [
				{"modelName": "GLM-4.5", "totalTokens": 21, "sortOrder": 1}
			]
		}
	}`)

	usage, err := extractGLMQuotaUsage(body, now)
	if err != nil {
		t.Fatalf("extractGLMQuotaUsage returned error: %v", err)
	}
	if usage.WeeklyUsedTokens != 21 {
		t.Fatalf("WeeklyUsedTokens = %d, want 21", usage.WeeklyUsedTokens)
	}
	if usage.FiveHourUsedTokens != 20 {
		t.Fatalf("FiveHourUsedTokens = %d, want 20", usage.FiveHourUsedTokens)
	}
	if len(usage.ModelSummary) != 1 || usage.ModelSummary[0].TotalTokens != 21 {
		t.Fatalf("ModelSummary = %#v", usage.ModelSummary)
	}
}
