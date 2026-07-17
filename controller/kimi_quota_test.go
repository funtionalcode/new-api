package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractKimiQuotaUsageParsesAccountInfo(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"data": {
			"cur": 5000000,
			"voucher_cur": 1498435,
			"acc": 5000000,
			"voucher_acc": 1500000,
			"voucher_expired": 0,
			"recharge_bonus_percent": 0,
			"use": 1565
		},
		"message": "Ok"
	}`)

	usage, err := extractKimiQuotaUsage(body)
	require.NoError(t, err)
	assert.Equal(t, int64(5_000_000), usage.CurrentQuota)
	assert.Equal(t, int64(1_498_435), usage.VoucherCurrentQuota)
	assert.Equal(t, int64(6_498_435), usage.RemainingQuota)
	assert.Equal(t, int64(6_500_000), usage.TotalQuota)
	assert.Equal(t, int64(1_565), usage.UsedQuota)
	assert.Equal(t, 100, usage.RemainingPercent)
}

func TestExtractKimiQuotaUsageRejectsProviderError(t *testing.T) {
	body := []byte(`{"code": 401, "message": "Unauthorized"}`)

	_, err := extractKimiQuotaUsage(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")
}

func TestBuildKimiQuotaCurlRequestParsesHeadersAndCookie(t *testing.T) {
	rawCurl := `curl 'https://platform.kimi.com/api?endpoint=organizationAccountInfo&oid=org-test' \
  -H 'accept: */*' \
  -H 'authorization: Bearer token-value' \
  -b 'INGRESSCOOKIE=abc; mintlify-auth-key=def' \
  --proxy 'http://127.0.0.1:7990'`

	requestConfig, err := buildKimiQuotaCurlRequest(rawCurl)
	require.NoError(t, err)
	assert.Equal(t, "https://platform.kimi.com/api?endpoint=organizationAccountInfo&oid=org-test", requestConfig.URL)
	assert.Equal(t, "Bearer token-value", requestConfig.Headers["authorization"])
	assert.Equal(t, "INGRESSCOOKIE=abc; mintlify-auth-key=def", requestConfig.Headers["Cookie"])
	assert.Equal(t, "http://127.0.0.1:7990", requestConfig.Proxy)
}

func TestSanitizeKimiQuotaBindingForRoleKeepsSensitiveConfigForAdminOnly(t *testing.T) {
	adminBinding := &model.KimiQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeKimiQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	assert.Equal(t, "curl secret", adminBinding.RequestCurl)
	assert.Equal(t, "socks5://user:pass@example.com:1080", adminBinding.Proxy)

	userBinding := &model.KimiQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeKimiQuotaBindingForRole(userBinding, 0)
	assert.Empty(t, userBinding.RequestCurl)
	assert.Empty(t, userBinding.Proxy)
}
