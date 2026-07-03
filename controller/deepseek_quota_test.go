package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestExtractDeepSeekQuotaUsageParsesSummary(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"msg": "",
		"data": {
			"biz_code": 0,
			"biz_msg": "",
			"biz_data": {
				"current_token": 10000000,
				"monthly_usage": "4354",
				"total_usage": 0,
				"normal_wallets": [
					{
						"currency": "CNY",
						"balance": "108.3185301600000000",
						"token_estimation": "36106176"
					}
				],
				"bonus_wallets": [
					{
						"currency": "CNY",
						"balance": "0",
						"token_estimation": "0"
					}
				],
				"total_available_token_estimation": "36106176",
				"monthly_costs": [
					{
						"currency": "CNY",
						"amount": "0.0091050000000000"
					}
				],
				"monthly_token_usage": "4354"
			}
		}
	}`)

	usage, err := extractDeepSeekQuotaUsage(body)
	if err != nil {
		t.Fatalf("extractDeepSeekQuotaUsage returned error: %v", err)
	}
	if usage.MonthlyLimitTokens != 10000000 {
		t.Fatalf("MonthlyLimitTokens = %d, want 10000000", usage.MonthlyLimitTokens)
	}
	if usage.MonthlyUsedTokens != 4354 {
		t.Fatalf("MonthlyUsedTokens = %d, want 4354", usage.MonthlyUsedTokens)
	}
	if usage.MonthlyRemainingTokens != 9995646 {
		t.Fatalf("MonthlyRemainingTokens = %d, want 9995646", usage.MonthlyRemainingTokens)
	}
	if usage.TotalAvailableTokens != 36106176 {
		t.Fatalf("TotalAvailableTokens = %d, want 36106176", usage.TotalAvailableTokens)
	}
	if len(usage.NormalWallets) != 1 || usage.NormalWallets[0].Balance != "108.3185301600000000" {
		t.Fatalf("NormalWallets = %#v", usage.NormalWallets)
	}
	if len(usage.MonthlyCosts) != 1 || usage.MonthlyCosts[0].Amount != "0.0091050000000000" {
		t.Fatalf("MonthlyCosts = %#v", usage.MonthlyCosts)
	}
}

func TestBuildDeepSeekQuotaCurlRequestParsesHeadersAndCookie(t *testing.T) {
	rawCurl := `curl 'https://platform.deepseek.com/api/v0/users/get_user_summary' \
  -H 'accept: */*' \
  -H 'authorization: Bearer token-value' \
  -H 'x-client-platform: web' \
  -b 'HWWAFSESID=abc; HWWAFSESTIME=123'`

	requestConfig, err := buildDeepSeekQuotaCurlRequest(rawCurl)
	if err != nil {
		t.Fatalf("buildDeepSeekQuotaCurlRequest returned error: %v", err)
	}
	if requestConfig.URL != "https://platform.deepseek.com/api/v0/users/get_user_summary" {
		t.Fatalf("URL = %q", requestConfig.URL)
	}
	if requestConfig.Headers["authorization"] != "Bearer token-value" {
		t.Fatalf("authorization header = %q", requestConfig.Headers["authorization"])
	}
	if requestConfig.Headers["Cookie"] != "HWWAFSESID=abc; HWWAFSESTIME=123" {
		t.Fatalf("cookie header = %q", requestConfig.Headers["Cookie"])
	}
	if requestConfig.Headers["x-client-platform"] != "web" {
		t.Fatalf("x-client-platform header = %q", requestConfig.Headers["x-client-platform"])
	}
}

func TestSanitizeDeepSeekQuotaBindingForRoleKeepsSensitiveConfigForAdminOnly(t *testing.T) {
	adminBinding := &model.DeepSeekQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeDeepSeekQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	if adminBinding.RequestCurl != "curl secret" {
		t.Fatalf("admin RequestCurl = %q", adminBinding.RequestCurl)
	}
	if adminBinding.Proxy != "socks5://user:pass@example.com:1080" {
		t.Fatalf("admin Proxy = %q", adminBinding.Proxy)
	}

	userBinding := &model.DeepSeekQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeDeepSeekQuotaBindingForRole(userBinding, 0)
	if userBinding.RequestCurl != "" {
		t.Fatalf("user RequestCurl = %q, want empty", userBinding.RequestCurl)
	}
	if userBinding.Proxy != "" {
		t.Fatalf("user Proxy = %q, want empty", userBinding.Proxy)
	}
}
