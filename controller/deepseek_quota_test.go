package controller

import (
	"net/url"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDeepSeekQuotaUsageParsesCurrentEndpoints(t *testing.T) {
	summaryBody := []byte(`{
		"code": 0,
		"msg": "",
		"data": {
			"biz_code": 0,
			"biz_msg": "",
			"biz_data": {
				"current_token": 100,
				"normal_wallets": [{"currency": "CNY", "balance": "85.1", "token_estimation": "0"}],
				"bonus_wallets": [{"currency": "CNY", "balance": "0", "token_estimation": "0"}],
				"total_costs": [{"currency": "CNY", "amount": "64.9"}]
			}
		}
	}`)
	amountBody := []byte(`{
		"code": 0,
		"msg": "",
		"data": {
			"biz_code": 0,
			"biz_msg": "",
			"biz_data": {
				"start": 100,
				"end": 200,
				"series": [
					{"buckets": [{"usage": {"RESPONSE_TOKEN": 5, "REQUEST": 2, "PROMPT_CACHE_HIT_TOKEN": 7, "PROMPT_CACHE_MISS_TOKEN": 11}}]},
					{"buckets": [{"usage": {"RESPONSE_TOKEN": 13, "REQUEST": 3, "PROMPT_CACHE_HIT_TOKEN": 17, "PROMPT_CACHE_MISS_TOKEN": 19}}]}
				]
			}
		}
	}`)
	costBody := []byte(`{
		"code": 0,
		"msg": "",
		"data": {
			"biz_code": 0,
			"biz_msg": "",
			"biz_data": {
				"start": 100,
				"end": 200,
				"data": [{
					"currency": "CNY",
					"series": [
						{"buckets": [{"cost": "0.1"}, {"cost": "0.2"}]},
						{"buckets": [{"cost": "0.3"}]}
					]
				}]
			}
		}
	}`)

	usage, err := extractDeepSeekQuotaUsage(summaryBody, amountBody, costBody)
	require.NoError(t, err)
	require.Len(t, usage.NormalWallets, 1)
	require.Len(t, usage.TotalCosts, 1)
	require.Len(t, usage.MonthlyCosts, 1)
	assert.Equal(t, "85.1", usage.NormalWallets[0].Balance)
	assert.Equal(t, "64.9", usage.TotalCosts[0].Amount)
	assert.Equal(t, "0.6", usage.MonthlyCosts[0].Amount)
	assert.Equal(t, int64(5), usage.RequestCount)
	assert.Equal(t, int64(72), usage.MonthlyUsedTokens)
	assert.Equal(t, int64(28), usage.MonthlyRemainingTokens)
	assert.Equal(t, 72, usage.MonthlyPercent)
}

func TestExtractDeepSeekQuotaUsageKeepsLegacySummaryCompatibility(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"msg": "",
		"data": {
			"biz_code": 0,
			"biz_msg": "",
			"biz_data": {
				"current_token": 10000000,
				"monthly_token_usage": "4354",
				"total_available_token_estimation": "36106176",
				"daily_token_usage": "123456",
				"monthly_costs": [{"currency": "CNY", "amount": "0.009105"}],
				"today_costs": [{"currency": "CNY", "amount": "0.001"}]
			}
		}
	}`)

	usage, err := extractDeepSeekQuotaUsage(body, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(10000000), usage.MonthlyLimitTokens)
	assert.Equal(t, int64(4354), usage.MonthlyUsedTokens)
	assert.Equal(t, int64(9995646), usage.MonthlyRemainingTokens)
	assert.Equal(t, int64(36106176), usage.TotalAvailableTokens)
	assert.Equal(t, int64(123456), usage.TodayUsedTokens)
	require.Len(t, usage.MonthlyCosts, 1)
	assert.Equal(t, "0.009105", usage.MonthlyCosts[0].Amount)
}

func TestBuildDeepSeekQuotaCurlRequestParsesHeadersAndCookie(t *testing.T) {
	rawCurl := `curl 'https://platform.deepseek.com/api/v0/users/get_user_summary' \
  -H 'accept: */*' \
  -H 'authorization: Bearer token-value' \
  -H 'x-client-platform: web' \
  -b 'HWWAFSESID=abc; HWWAFSESTIME=123' \
  --proxy 'http://127.0.0.1:7990'`

	requestConfig, err := buildDeepSeekQuotaCurlRequest(rawCurl)
	require.NoError(t, err)
	assert.Equal(t, "https://platform.deepseek.com/api/v0/users/get_user_summary", requestConfig.URL)
	assert.Equal(t, "Bearer token-value", requestConfig.Headers["authorization"])
	assert.Equal(t, "HWWAFSESID=abc; HWWAFSESTIME=123", requestConfig.Headers["Cookie"])
	assert.Equal(t, "web", requestConfig.Headers["x-client-platform"])
	assert.Equal(t, "http://127.0.0.1:7990", requestConfig.Proxy)
}

func TestBuildDeepSeekQuotaCurlRequestParsesInlineProxy(t *testing.T) {
	rawCurl := `curl -xsocks5://127.0.0.1:7990 'https://platform.deepseek.com/api/v0/users/get_user_summary'`

	requestConfig, err := buildDeepSeekQuotaCurlRequest(rawCurl)
	require.NoError(t, err)
	assert.Equal(t, "socks5://127.0.0.1:7990", requestConfig.Proxy)
}

func TestBuildDeepSeekQuotaUsageCurlRequestRefreshesRollingRange(t *testing.T) {
	rawCurl := `curl 'https://platform.deepseek.com/api/v0/usage/by_api_key/amount?start=1&end=2&tz=8&scope=all'`
	now := time.Date(2026, time.August, 6, 10, 30, 0, 0, time.UTC)

	requestConfig, err := buildDeepSeekQuotaUsageCurlRequest(rawCurl, now)
	require.NoError(t, err)
	parsedURL, err := url.Parse(requestConfig.URL)
	require.NoError(t, err)
	assert.Equal(t, "1783468800", parsedURL.Query().Get("start"))
	assert.Equal(t, "1786060800", parsedURL.Query().Get("end"))
	assert.Equal(t, "0", parsedURL.Query().Get("tz"))
	assert.Equal(t, "all", parsedURL.Query().Get("scope"))
}

func TestSanitizeDeepSeekQuotaBindingForRoleKeepsSensitiveConfigForAdminOnly(t *testing.T) {
	adminBinding := &model.DeepSeekQuotaBinding{
		RequestCurl:     "curl summary-secret",
		UsageAmountCurl: "curl amount-secret",
		UsageCostCurl:   "curl cost-secret",
		Proxy:           "socks5://user:pass@example.com:1080",
	}
	sanitizeDeepSeekQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	assert.Equal(t, "curl summary-secret", adminBinding.RequestCurl)
	assert.Equal(t, "curl amount-secret", adminBinding.UsageAmountCurl)
	assert.Equal(t, "curl cost-secret", adminBinding.UsageCostCurl)
	assert.Equal(t, "socks5://user:pass@example.com:1080", adminBinding.Proxy)

	userBinding := &model.DeepSeekQuotaBinding{
		RequestCurl:     "curl summary-secret",
		UsageAmountCurl: "curl amount-secret",
		UsageCostCurl:   "curl cost-secret",
		Proxy:           "socks5://user:pass@example.com:1080",
	}
	sanitizeDeepSeekQuotaBindingForRole(userBinding, 0)
	assert.Empty(t, userBinding.RequestCurl)
	assert.Empty(t, userBinding.UsageAmountCurl)
	assert.Empty(t, userBinding.UsageCostCurl)
	assert.Empty(t, userBinding.Proxy)
}
