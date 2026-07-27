package controller

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGLMQuotaUsageRequestParsesCurlAndRewritesWindow(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	rawCurl := `curl 'https://bigmodel.cn/api/monitor/usage/model-usage?startTime=2026-06-27+00:00:00&endTime=2026-07-03+23:59:59&type=3&refer__1090=abc' \
  -H 'accept: application/json, text/plain, */*' \
  -H 'authorization: token-value' \
  -H 'bigmodel-project: proj_1' \
  -b 'session=abc; token=def' \
  --proxy=http://127.0.0.1:7990`

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
	if requestConfig.Proxy != "http://127.0.0.1:7990" {
		t.Fatalf("proxy = %q", requestConfig.Proxy)
	}
}

func TestBuildGLMQuotaUsageRequestParsesInlineProxy(t *testing.T) {
	now := time.Date(2026, 7, 3, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	rawCurl := `curl -xsocks5h://127.0.0.1:7990 'https://bigmodel.cn/api/monitor/usage/model-usage?type=3'`

	requestConfig, err := buildGLMQuotaUsageRequest(rawCurl, now)
	if err != nil {
		t.Fatalf("buildGLMQuotaUsageRequest returned error: %v", err)
	}
	if requestConfig.Proxy != "socks5h://127.0.0.1:7990" {
		t.Fatalf("proxy = %q", requestConfig.Proxy)
	}
}

func TestBuildGLMQuotaLimitRequestUsesQuotaLimitEndpoint(t *testing.T) {
	rawCurl := `curl 'https://bigmodel.cn/api/monitor/usage/model-usage?startTime=2026-06-27+00:00:00&endTime=2026-07-03+23:59:59&type=3&refer__1090=abc' \
  -H 'authorization: token-value' \
  -H 'bigmodel-organization: org_1' \
  -H 'bigmodel-project: proj_1' \
  -H 'cookie: header_cookie=1' \
  -b 'session=abc; token=def'`

	requestConfig, err := buildGLMQuotaLimitRequest(rawCurl)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestConfig.URL)
	require.NoError(t, err)
	assert.Equal(t, "/api/monitor/usage/quota/limit", parsedURL.Path)
	assert.Equal(t, "2", parsedURL.Query().Get("type"))
	assert.Equal(t, "abc", parsedURL.Query().Get("refer__1090"))
	assert.Equal(t, "token-value", requestConfig.Headers["authorization"])
	assert.Equal(t, "org_1", requestConfig.Headers["bigmodel-organization"])
	assert.Equal(t, "proj_1", requestConfig.Headers["bigmodel-project"])
	assert.Equal(t, "header_cookie=1; session=abc; token=def", glmQuotaTestHeaderValue(requestConfig.Headers, "cookie"))
}

func glmQuotaTestHeaderValue(headers map[string]string, key string) string {
	for headerKey, value := range headers {
		if strings.EqualFold(headerKey, key) {
			return value
		}
	}
	return ""
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

func TestApplyGLMQuotaPlanSpecDefaultsAdvanced(t *testing.T) {
	update := model.GLMQuotaBindingUpdate{PlanType: "高级版"}

	applyGLMQuotaPlanSpecDefaults(&update)

	if update.PlanType != "高级版" {
		t.Fatalf("PlanType = %q, want 高级版", update.PlanType)
	}
	if update.FiveHourLimitTokens != 160000000 {
		t.Fatalf("FiveHourLimitTokens = %d, want 160000000", update.FiveHourLimitTokens)
	}
	if update.WeeklyLimitTokens != 800000000 {
		t.Fatalf("WeeklyLimitTokens = %d, want 800000000", update.WeeklyLimitTokens)
	}
}

func TestSanitizeGLMQuotaBindingForRoleKeepsSensitiveConfigForAdminOnly(t *testing.T) {
	adminBinding := &model.GLMQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeGLMQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	if adminBinding.RequestCurl != "curl secret" {
		t.Fatalf("admin RequestCurl = %q", adminBinding.RequestCurl)
	}
	if adminBinding.Proxy != "socks5://user:pass@example.com:1080" {
		t.Fatalf("admin Proxy = %q", adminBinding.Proxy)
	}

	userBinding := &model.GLMQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeGLMQuotaBindingForRole(userBinding, 0)
	if userBinding.RequestCurl != "" {
		t.Fatalf("user RequestCurl = %q, want empty", userBinding.RequestCurl)
	}
	if userBinding.Proxy != "" {
		t.Fatalf("user Proxy = %q, want empty", userBinding.Proxy)
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

func TestExtractGLMQuotaLimitKeepsResetTimesAndMCPQuota(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"msg": "操作成功",
		"success": true,
		"data": {
			"limits": [
				{
					"type": "TOKENS_LIMIT",
					"unit": 3,
					"percentage": 30,
					"nextResetTime": 1783065600000,
					"currentValue": 123
				},
				{
					"type": "TOKENS_LIMIT",
					"unit": 6,
					"percentage": 73,
					"nextResetTime": 1783389960000,
					"currentValue": 456
				},
				{
					"type": "TIME_LIMIT",
					"unit": 5,
					"percentage": 1,
					"nextResetTime": 1785745560000,
					"currentValue": 2,
					"usage": 1000
				}
			]
		}
	}`)

	usage, err := extractGLMQuotaLimit(body)
	require.NoError(t, err)

	assert.Equal(t, 30, usage.FiveHourPercent)
	assert.True(t, usage.HasFiveHourPercent)
	assert.Equal(t, int64(1783065600), usage.FiveHourResetAt)
	assert.Equal(t, 73, usage.WeeklyPercent)
	assert.True(t, usage.HasWeeklyPercent)
	assert.Equal(t, int64(1783389960), usage.WeeklyResetAt)
	assert.Equal(t, int64(2), usage.MCPMonthlyUsed)
	assert.Equal(t, int64(1000), usage.MCPMonthlyLimit)
	assert.Equal(t, 1, usage.MCPMonthlyPercent)
	assert.Equal(t, int64(1785745560), usage.MCPMonthlyResetAt)
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
