package controller

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

type cliproxyAntigravityQuotaBucket struct {
	BucketID          string   `json:"bucket_id"`
	RemainingFraction *float64 `json:"remaining_fraction,omitempty"`
	ResetAt           int64    `json:"reset_at"`
	Disabled          bool     `json:"disabled,omitempty"`
}

func fetchCliproxyAntigravityPlan(ctx context.Context, caller cliproxyAPICaller, authIndex string) string {
	result, err := caller.CallAPI(ctx, service.CliproxyAPICallRequest{
		AuthIndex: authIndex,
		Method:    http.MethodPost,
		URL:       "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		Header: map[string]string{
			"Authorization": "Bearer $TOKEN$",
			"Content-Type":  "application/json",
			"User-Agent":    "antigravity/cli/1.0.13 (aidev_client; os_type=darwin; arch=arm64)",
		},
		Data: `{"metadata":{"ideType":"ANTIGRAVITY"}}`,
	})
	if err != nil || result == nil || result.Status >= http.StatusBadRequest || result.StatusCode >= http.StatusBadRequest {
		return ""
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	// 付费账号的 currentTier 仍可能是 free-tier，优先使用 paidTier。
	for _, key := range []string{"paidTier", "currentTier"} {
		tier := mapFromMap(body, key)
		id := stringFromMap(tier, "id")
		if id == "free-tier" {
			return "Free"
		}
		if plan := firstNonEmpty(stringFromMap(tier, "name"), id); plan != "" {
			return plan
		}
	}
	return ""
}

func isCliproxyAntigravityAuthFile(binding *model.CliproxyAuthFileBinding) bool {
	if binding == nil {
		return false
	}
	if binding.Provider != "" {
		return normalizeCliproxyPlan(binding.Provider) == "antigravity"
	}
	return normalizeCliproxyPlan(binding.LastPlanType) == "antigravity" ||
		hasCliproxyAuthFileNamePrefix(binding.AuthName, "antigravity") ||
		hasCliproxyAuthFileNamePrefix(binding.AuthFile, "antigravity")
}

func extractCliproxyAntigravityUsage(body map[string]any) (cliproxyUsageRefreshBody, error) {
	var response struct {
		Groups []struct {
			Buckets []struct {
				BucketID          string   `json:"bucketId"`
				RemainingFraction *float64 `json:"remainingFraction"`
				Remaining         *struct {
					Fraction *float64 `json:"remainingFraction"`
				} `json:"remaining"`
				ResetTime string `json:"resetTime"`
				Disabled  bool   `json:"disabled"`
			} `json:"buckets"`
		} `json:"groups"`
	}
	raw, err := common.Marshal(body)
	if err != nil {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度数据无效: %w", err)
	}
	if err := common.Unmarshal(raw, &response); err != nil {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度数据无效: %w", err)
	}
	buckets := make(map[string]cliproxyAntigravityQuotaBucket)
	for _, group := range response.Groups {
		for _, bucket := range group.Buckets {
			switch bucket.BucketID {
			case "gemini-5h", "gemini-weekly", "3p-5h", "3p-weekly":
			default:
				continue
			}
			if _, exists := buckets[bucket.BucketID]; exists {
				return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度窗口重复: %s", bucket.BucketID)
			}
			fraction := bucket.RemainingFraction
			if fraction == nil && bucket.Remaining != nil {
				fraction = bucket.Remaining.Fraction
			}
			if fraction != nil && (math.IsNaN(*fraction) || math.IsInf(*fraction, 0) || *fraction < 0 || *fraction > 1) {
				return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 剩余额度超出范围: %s", bucket.BucketID)
			}
			var resetAt int64
			if bucket.ResetTime != "" {
				reset, err := time.Parse(time.RFC3339, bucket.ResetTime)
				if err != nil {
					return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 重置时间无效: %s", bucket.BucketID)
				}
				resetAt = reset.Unix()
			}
			buckets[bucket.BucketID] = cliproxyAntigravityQuotaBucket{BucketID: bucket.BucketID, RemainingFraction: fraction, ResetAt: resetAt, Disabled: bucket.Disabled}
		}
	}
	if len(buckets) == 0 {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 刷新结果缺少额度窗口")
	}
	ordered := make([]cliproxyAntigravityQuotaBucket, 0, len(buckets))
	for _, id := range []string{"gemini-5h", "gemini-weekly", "3p-5h", "3p-weekly"} {
		if bucket, ok := buckets[id]; ok {
			ordered = append(ordered, bucket)
		}
	}
	raw, err = common.Marshal(ordered)
	if err != nil {
		return cliproxyUsageRefreshBody{}, err
	}
	return cliproxyUsageRefreshBody{PlanType: "antigravity", AntigravityQuota: string(raw)}, nil
}
