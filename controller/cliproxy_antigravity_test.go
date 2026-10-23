package controller

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cliproxyAntigravityQuotaSummaryFixture = `{
  "groups": [
    {
      "displayName": "Gemini Models",
      "buckets": [
        {"bucketId":"gemini-weekly","window":"weekly","remainingFraction":0.95260817,"resetTime":"2026-10-26T02:56:14Z"},
        {"bucketId":"gemini-5h","window":"5h","remainingFraction":1,"resetTime":"2026-10-23T11:48:03Z"}
      ]
    },
    {
      "displayName": "Claude and GPT models",
      "buckets": [
        {"bucketId":"3p-weekly","window":"weekly","remainingFraction":1,"resetTime":"2026-10-27T03:21:48Z"},
        {"bucketId":"3p-5h","window":"5h","remainingFraction":1,"resetTime":"2026-10-23T11:48:03Z"}
      ]
    }
  ]
}`

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
		assert.Equal(t, "application/json", request.Header["Content-Type"])
		assert.Equal(t, "antigravity/hub/2.23.0 darwin/arm64", request.Header["User-Agent"])
		assert.JSONEq(t, `{"project":"aicode-consumers"}`, request.Data)
	}
	t.Run("明确提供商优先于文件名前缀", func(t *testing.T) {
		request := buildCliproxyUsageRefreshRequest(&model.CliproxyAuthFileBinding{Provider: "codex", AuthName: "antigravity-account.json"})
		assert.Equal(t, "https://chatgpt.com/backend-api/wham/usage", request.URL)
	})
}

func TestExtractCliproxyUsagePreservesAntigravityQuotaGroups(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		wantWeekly float64
	}{
		{"current summary string body", cliproxyAntigravityQuotaSummaryFixture, 0.95260817},
		{"explicit zero stays exhausted", strings.Replace(cliproxyAntigravityQuotaSummaryFixture, `"remainingFraction":0.95260817`, `"remainingFraction":0`, 1), 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := common.Marshal(map[string]any{"status_code": 200, "body": tt.body})
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
			for i, expected := range []float64{1, tt.wantWeekly, 1, 1} {
				require.NotNil(t, buckets[i].RemainingFraction)
				assert.Equal(t, expected, *buckets[i].RemainingFraction)
			}
			assert.Equal(t, time.Date(2026, 10, 23, 11, 48, 3, 0, time.UTC).Unix(), buckets[0].ResetAt)
			assert.Equal(t, time.Date(2026, 10, 26, 2, 56, 14, 0, time.UTC).Unix(), buckets[1].ResetAt)
			assert.Equal(t, buckets[0].ResetAt, buckets[2].ResetAt)
			assert.Equal(t, time.Date(2026, 10, 27, 3, 21, 48, 0, time.UTC).Unix(), buckets[3].ResetAt)
		})
	}
}

func TestExtractAntigravityUsageHandlesUnavailableAndInvalidBuckets(t *testing.T) {
	for _, tt := range []struct {
		name, body, wantError string
	}{
		{"disabled", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","disabled":true}]}]}`, "已禁用"},
		{"unknown remaining", `{"groups":[{"buckets":[{"bucketId":"gemini-5h"}]}]}`, "缺少剩余额度"},
		{"nested fraction", strings.Replace(cliproxyAntigravityQuotaSummaryFixture, `"remainingFraction":0.95260817`, `"remaining":{"remainingFraction":0.95260817}`, 1), "缺少剩余额度: gemini-weekly"},
		{"single Gemini window", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.5}]}]}`, "缺少额度窗口: gemini-weekly"},
		{"only third-party windows", `{"groups":[{"buckets":[{"bucketId":"3p-5h","remainingFraction":0.5},{"bucketId":"3p-weekly","remainingFraction":1}]}]}`, "缺少额度窗口: gemini-5h"},
		{"duplicate window", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.5},{"bucketId":"gemini-5h","remainingFraction":0.4}]}]}`, "窗口重复"},
		{"empty", `{"groups":[]}`, "缺少额度窗口"},
		{"unknown windows only", `{"groups":[{"buckets":[{"bucketId":"future-5h","remainingFraction":1}]}]}`, "缺少额度窗口"},
		{"model response is not quota summary", `{"models":{"gemini-pro":{"quotaInfo":{"remainingFraction":1}}}}`, "缺少额度窗口"},
		{"invalid fraction", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":2}]}]}`, "超出范围"},
		{"invalid date", `{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":1,"resetTime":"invalid"}]}]}`, "重置时间无效"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.UnmarshalJsonStr(tt.body, &body))
			_, err := extractCliproxyAntigravityUsage(body)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}
