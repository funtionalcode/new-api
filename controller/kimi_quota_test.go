package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
		RequestCurl:  "curl secret",
		RefreshToken: "refresh-secret",
		Proxy:        "socks5://user:pass@example.com:1080",
	}
	sanitizeKimiQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	assert.Equal(t, "curl secret", adminBinding.RequestCurl)
	assert.Equal(t, "refresh-secret", adminBinding.RefreshToken)
	assert.Equal(t, "socks5://user:pass@example.com:1080", adminBinding.Proxy)

	userBinding := &model.KimiQuotaBinding{
		RequestCurl:  "curl secret",
		RefreshToken: "refresh-secret",
		Proxy:        "socks5://user:pass@example.com:1080",
	}
	sanitizeKimiQuotaBindingForRole(userBinding, 0)
	assert.Empty(t, userBinding.RequestCurl)
	assert.Empty(t, userBinding.RefreshToken)
	assert.Empty(t, userBinding.Proxy)
}

func TestReplaceKimiQuotaAccessTokenInCurl(t *testing.T) {
	rawCurl := `curl 'https://platform.kimi.com/api?endpoint=organizationAccountInfo' -H 'authorization: Bearer old-token'`
	updated, err := replaceKimiQuotaAccessTokenInCurl(rawCurl, "new-token")
	require.NoError(t, err)
	assert.Equal(t, `curl 'https://platform.kimi.com/api?endpoint=organizationAccountInfo' -H 'authorization: Bearer new-token'`, updated)
}

func TestBuildKimiQuotaRefreshTokenURL(t *testing.T) {
	refreshURL, err := buildKimiQuotaRefreshTokenURL("https://platform.kimi.com/api?endpoint=organizationAccountInfo&oid=org-test")
	require.NoError(t, err)
	assert.Equal(t, "https://platform.kimi.com/api?endpoint=refreshToken", refreshURL)
}

func TestIsKimiQuotaAuthFailure(t *testing.T) {
	assert.True(t, isKimiQuotaAuthFailure(http.StatusUnauthorized, []byte(`{}`)))
	assert.True(t, isKimiQuotaAuthFailure(http.StatusOK, []byte(`{"code":401,"message":"Unauthorized"}`)))
	assert.False(t, isKimiQuotaAuthFailure(http.StatusOK, []byte(`{"code":0,"message":"Ok"}`)))
}

func TestRefreshKimiQuotaUsageRefreshesTokenOn401(t *testing.T) {
	useKimiQuotaControllerTestDB(t)

	accountCalls := 0
	refreshCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.RawQuery, "endpoint=refreshToken"):
			refreshCalls++
			assert.Equal(t, "refresh-token-value", r.Header.Get("msh-authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":0,"message":"success","data":{"access_token":"new-access-token","refresh_token":"new-refresh-token"}}`)
		case strings.Contains(r.URL.RawQuery, "endpoint=organizationAccountInfo"):
			accountCalls++
			auth := r.Header.Get("Authorization")
			if auth != "Bearer new-access-token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"code":401,"message":"Unauthorized"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{
				"code": 0,
				"message": "Ok",
				"data": {
					"cur": 100,
					"voucher_cur": 20,
					"acc": 100,
					"voucher_acc": 20,
					"voucher_expired": 0,
					"recharge_bonus_percent": 0,
					"use": 1
				}
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	binding := &model.KimiQuotaBinding{
		Name: "kimi",
		RequestCurl: "curl " + server.URL + "/api?endpoint=organizationAccountInfo&oid=org-test " +
			`-H 'authorization: Bearer old-access-token'`,
		RefreshToken: "refresh-token-value",
		Enabled:      true,
	}
	require.NoError(t, model.CreateKimiQuotaBinding(binding))

	usage, err := refreshKimiQuotaUsage(context.Background(), binding)
	require.NoError(t, err)
	assert.Equal(t, int64(120), usage.RemainingQuota)
	assert.Equal(t, 2, accountCalls)
	assert.Equal(t, 1, refreshCalls)

	stored, err := model.GetKimiQuotaBindingById(binding.Id)
	require.NoError(t, err)
	assert.Contains(t, stored.RequestCurl, "Bearer new-access-token")
	assert.Contains(t, stored.RequestCurl, "authorization: Bearer new-access-token'")
	assert.Equal(t, "new-refresh-token", stored.RefreshToken)
}

func useKimiQuotaControllerTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	t.Cleanup(func() {
		model.DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.KimiQuotaBinding{}))
	model.DB = db
}
