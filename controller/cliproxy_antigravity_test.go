package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCliproxyUsageRefreshRequestUsesAntigravityQuotaSummary(t *testing.T) {
	for _, binding := range []*model.CliproxyAuthFileBinding{
		{AuthIndex: "ag", AuthName: "antigravity-test@example.com.json", LastPlanType: "oauth"},
		{AuthIndex: "ag", AuthFile: `C:\auth\antigravity_account.json`},
		{AuthIndex: "ag", Provider: "antigravity", AuthName: "account.json"},
	} {
		request := buildCliproxyUsageRefreshRequest(binding)
		assert.Equal(t, "ag", request.AuthIndex)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", request.URL)
		assert.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
		assert.Equal(t, "antigravity/cli/1.0.13 (aidev_client; os_type=darwin; arch=arm64)", request.Header["User-Agent"])
		assert.JSONEq(t, `{"project":"aicode-consumers"}`, request.Data)
	}
	t.Run("明确提供商优先于文件名前缀", func(t *testing.T) {
		request := buildCliproxyUsageRefreshRequest(&model.CliproxyAuthFileBinding{Provider: "codex", AuthName: "antigravity-account.json"})
		assert.Equal(t, "https://chatgpt.com/backend-api/wham/usage", request.URL)
	})
}

func TestExtractAntigravityModelQuotaPreservesConservativeGroups(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
	}{
		{
			name: "分组取最低剩余比例和限制模型的最晚重置时间",
			body: `{"models":{"gemini-flash":{"quotaInfo":{"remainingFraction":0.8,"resetTime":"2026-10-10T08:00:00Z"}},"gemini-pro":{"quotaInfo":{"remainingFraction":0.2,"resetTime":"2026-10-09T08:00:00Z"}},"gemini-agent":{"quotaInfo":{"remainingFraction":0.2,"resetTime":"2026-10-09T09:00:00Z"}},"claude-sonnet":{"quotaInfo":{"remainingFraction":1}},"gpt-oss":{"quotaInfo":{"remainingFraction":0}},"tab_flash_lite_preview":{"quotaInfo":{"remainingFraction":0}},"gemini-no-quota":{}}}`,
			want: `[{"bucket_id":"gemini-shared","remaining_fraction":0.2,"reset_at":1791536400},{"bucket_id":"3p-shared","remaining_fraction":0,"reset_at":0}]`,
		},
		{
			name: "限制模型缺少重置时间时保留未知状态",
			body: `{"models":{"gemini-a":{"quotaInfo":{"remainingFraction":0.1}},"gemini-b":{"quotaInfo":{"remainingFraction":0.1,"resetTime":"2026-10-09T09:00:00Z"}},"opaque":{"displayName":"Claude Sonnet","quotaInfo":{"remainingFraction":0.5}}}}`,
			want: `[{"bucket_id":"gemini-shared","remaining_fraction":0.1,"reset_at":0},{"bucket_id":"3p-shared","remaining_fraction":0.5,"reset_at":0}]`,
		},
		{
			name: "模型响应不覆盖已有窗口数据",
			body: `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.9}]}],"models":{"gemini-pro":{"quotaInfo":{"remainingFraction":0.1}}}}`,
			want: `[{"bucket_id":"gemini-5h","remaining_fraction":0.9,"reset_at":0}]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.UnmarshalJsonStr(tt.body, &body))
			usage, err := extractCliproxyAntigravityUsage(body)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, usage.AntigravityQuota)
		})
	}
}

func TestRefreshAntigravityQuotaStopsFallbackAfterRateLimit(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request service.CliproxyAPICallRequest
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		requests = append(requests, request.URL)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"status_code":429,"body":{}}`))
	}))
	t.Cleanup(upstream.Close)
	client, err := service.NewCliproxyAPIClient(upstream.URL, "test-password")
	require.NoError(t, err)
	_, err = refreshCliproxyAntigravityUsage(context.Background(), client, &model.CliproxyAuthFileBinding{Provider: "antigravity", AuthIndex: "ag"})
	require.ErrorContains(t, err, "429")
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, requests)
	for _, requestURL := range requests {
		assert.Equal(t, "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", requestURL)
	}
}

func TestExtractCliproxyUsagePreservesAntigravityQuotaGroups(t *testing.T) {
	body := `{"groups":[{"displayName":"Gemini Models","buckets":[{"bucketId":"gemini-weekly","remainingFraction":0.9986187,"resetTime":"2026-09-17T06:15:51Z"},{"bucketId":"gemini-5h","remainingFraction":0.9917125,"resetTime":"2026-09-10T11:15:51Z"}]},{"displayName":"Claude and GPT models","buckets":[{"bucketId":"3p-weekly","remainingFraction":1,"resetTime":"2026-09-17T06:32:26Z"},{"bucketId":"3p-5h","remainingFraction":0,"resetTime":"2026-09-10T11:32:26Z"}]}]}`
	raw, err := common.Marshal(map[string]any{"status_code": 200, "body": body})
	require.NoError(t, err)
	var response service.CliproxyAPICallResponse
	require.NoError(t, common.Unmarshal(raw, &response))
	usage, err := extractCliproxyUsage(&response)
	require.NoError(t, err)
	assert.Equal(t, "antigravity", usage.PlanType)
	var buckets []cliproxyAntigravityQuotaBucket
	require.NoError(t, common.UnmarshalJsonStr(usage.AntigravityQuota, &buckets))
	require.Len(t, buckets, 4)
	assert.Equal(t, []string{"gemini-5h", "gemini-weekly", "3p-5h", "3p-weekly"}, []string{buckets[0].BucketID, buckets[1].BucketID, buckets[2].BucketID, buckets[3].BucketID})
	for i, expected := range []float64{0.9917125, 0.9986187, 0, 1} {
		require.NotNil(t, buckets[i].RemainingFraction)
		assert.Equal(t, expected, *buckets[i].RemainingFraction)
	}
	assert.Equal(t, time.Date(2026, 9, 10, 11, 15, 51, 0, time.UTC).Unix(), buckets[0].ResetAt)
}

func TestExtractAntigravityUsageHandlesUnavailableAndInvalidBuckets(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		wantError  bool
	}{
		{"disabled", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","disabled":true}]}]}`, false},
		{"unknown remaining", `{"groups":[{"buckets":[{"bucketId":"gemini-5h"}]}]}`, false},
		{"nested fraction", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remaining":{"remainingFraction":0.5}}]}]}`, false},
		{"empty", `{"groups":[]}`, true},
		{"invalid fraction", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":2}]}]}`, true},
		{"invalid date", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":1,"resetTime":"invalid"}]}]}`, true},
		{"model missing fraction", `{"models":{"gemini-pro":{"quotaInfo":{"resetTime":"2026-10-09T09:00:00Z"}}}}`, true},
		{"model invalid fraction", `{"models":{"gemini-pro":{"quotaInfo":{"remainingFraction":-0.1}}}}`, true},
		{"model invalid date", `{"models":{"gemini-pro":{"quotaInfo":{"remainingFraction":0.1,"resetTime":"invalid"}}}}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.UnmarshalJsonStr(tt.body, &body))
			usage, err := extractCliproxyAntigravityUsage(body)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var buckets []cliproxyAntigravityQuotaBucket
			require.NoError(t, common.UnmarshalJsonStr(usage.AntigravityQuota, &buckets))
			require.Len(t, buckets, 1)
			if tt.name == "nested fraction" {
				require.NotNil(t, buckets[0].RemainingFraction)
				assert.Equal(t, 0.5, *buckets[0].RemainingFraction)
			} else {
				assert.Nil(t, buckets[0].RemainingFraction)
			}
			assert.Equal(t, tt.name == "disabled", buckets[0].Disabled)
		})
	}
}
