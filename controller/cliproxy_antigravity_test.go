package controller

import (
	"net/http"
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
