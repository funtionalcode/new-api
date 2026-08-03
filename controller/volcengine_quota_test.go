package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const volcengineQuotaSuccessFixture = `{
	"ResponseMetadata": {
		"RequestId": "request-id",
		"Action": "GetAgentPlanAFPUsage"
	},
	"Result": {
		"PlanType": "medium",
		"AFPFiveHour": {"Quota": 10000, "Used": 0.3736, "SubscribeTime": 1785746215000, "ResetTime": 1785764215000},
		"AFPDaily": {"Quota": 50000, "Used": 0, "SubscribeTime": 1785686400000, "ResetTime": 1785772800000},
		"AFPWeekly": {"Quota": 35000, "Used": 0.3736, "SubscribeTime": 1785686400000, "ResetTime": 1786291200000},
		"AFPMonthly": {"Quota": 100000, "Used": 0.3736, "SubscribeTime": 1785745776000, "ResetTime": 1788451199000}
	}
}`

func TestExtractVolcengineQuotaUsageParsesAFPWindows(t *testing.T) {
	usage, err := extractVolcengineQuotaUsage([]byte(volcengineQuotaSuccessFixture))
	require.NoError(t, err)

	assert.Equal(t, "medium", usage.PlanType)
	assert.Equal(t, int64(10_000), usage.FiveHour.Quota)
	assert.InDelta(t, 0.3736, usage.FiveHour.UsedAFP, 0.000001)
	assert.Equal(t, int64(1_785_746_215), usage.FiveHour.SubscribeTime)
	assert.Equal(t, int64(1_785_764_215), usage.FiveHour.ResetTime)
	assert.Equal(t, int64(50_000), usage.Daily.Quota)
	assert.Equal(t, int64(35_000), usage.Weekly.Quota)
	assert.Equal(t, int64(100_000), usage.Monthly.Quota)
	assert.Equal(t, int64(1_788_451_199), usage.Monthly.ResetTime)
}

func TestExtractVolcengineQuotaUsageRejectsHTTP200ProviderError(t *testing.T) {
	body := []byte(`{
		"ResponseMetadata": {
			"Error": {"Code": "InvalidCSRFToken", "Message": "Invalid CSRF token."}
		}
	}`)

	_, err := extractVolcengineQuotaUsage(body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidCSRFToken")
}

func TestBuildVolcengineQuotaCurlRequestPreservesPostBodyAndCredentials(t *testing.T) {
	rawCurl := `curl 'https://console.volcengine.com/api/top/ark/cn-beijing/2024-01-01/GetAgentPlanAFPUsage?' \
  -H 'content-type: application/json' \
  -H 'x-csrf-token: csrf-value' \
  -b 'session=fake-session' \
  --data-raw '{}'`

	requestConfig, err := buildVolcengineQuotaCurlRequest(rawCurl)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, requestConfig.Method)
	assert.Equal(t, "{}", requestConfig.Body)
	assert.Equal(t, "csrf-value", requestConfig.Headers["x-csrf-token"])
	assert.Equal(t, "session=fake-session", requestConfig.Headers["Cookie"])
}

func TestRefreshVolcengineQuotaUsageReplaysSavedCurl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "csrf-value", r.Header.Get("X-Csrf-Token"))
		assert.Equal(t, "session=fake-session", r.Header.Get("Cookie"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, volcengineQuotaSuccessFixture)
	}))
	defer server.Close()

	binding := &model.VolcengineQuotaBinding{
		RequestCurl: "curl '" + server.URL + "' -H 'content-type: application/json' " +
			"-H 'x-csrf-token: csrf-value' -b 'session=fake-session' --data-raw '{}'",
		Enabled: true,
	}
	usage, err := refreshVolcengineQuotaUsage(context.Background(), binding)
	require.NoError(t, err)
	assert.Equal(t, "medium", usage.PlanType)
	assert.Equal(t, int64(10_000), usage.FiveHour.Quota)
}

func TestSanitizeVolcengineQuotaBindingForRoleKeepsSensitiveConfigForAdminOnly(t *testing.T) {
	adminBinding := &model.VolcengineQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeVolcengineQuotaBindingForRole(adminBinding, common.RoleAdminUser)
	assert.Equal(t, "curl secret", adminBinding.RequestCurl)
	assert.Equal(t, "socks5://user:pass@example.com:1080", adminBinding.Proxy)

	userBinding := &model.VolcengineQuotaBinding{
		RequestCurl: "curl secret",
		Proxy:       "socks5://user:pass@example.com:1080",
	}
	sanitizeVolcengineQuotaBindingForRole(userBinding, 0)
	assert.Empty(t, userBinding.RequestCurl)
	assert.Empty(t, userBinding.Proxy)
}
