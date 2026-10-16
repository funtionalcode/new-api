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

func fetchCliproxyAntigravityPlan(ctx context.Context, caller cliproxyAPICaller, authIndex string) (string, error) {
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
	if err != nil {
		return "", fmt.Errorf("Antigravity 套餐查询失败: %w", err)
	}
	if result == nil {
		return "", fmt.Errorf("Antigravity 套餐查询结果为空")
	}
	if status := max(result.Status, result.StatusCode); status >= http.StatusBadRequest {
		return "", fmt.Errorf("Antigravity 套餐查询失败，上游状态码: %d", status)
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
			return "Free", nil
		}
		if plan := firstNonEmpty(stringFromMap(tier, "name"), id); plan != "" {
			return plan, nil
		}
	}
	return "", fmt.Errorf("Antigravity 查询结果缺少套餐信息")
}

func refreshCliproxyAntigravityUsage(ctx context.Context, caller cliproxyAPICaller, binding *model.CliproxyAuthFileBinding) (cliproxyUsageRefreshBody, error) {
	result, err := caller.CallAPI(ctx, buildCliproxyUsageRefreshRequest(binding))
	if err != nil {
		return cliproxyUsageRefreshBody{}, err
	}
	if result == nil {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度查询结果为空")
	}
	body := result.Body
	if len(body) == 0 {
		body = result.Data
	}
	usage, err := extractCliproxyAntigravityUsage(body)
	if err != nil {
		return cliproxyUsageRefreshBody{}, err
	}
	usage.PlanType, err = fetchCliproxyAntigravityPlan(ctx, caller, binding.AuthIndex)
	if err != nil {
		return cliproxyUsageRefreshBody{}, err
	}
	return usage, nil
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
			if bucket.Disabled {
				return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度窗口已禁用: %s", bucket.BucketID)
			}
			fraction := bucket.RemainingFraction
			if fraction == nil && bucket.Remaining != nil {
				fraction = bucket.Remaining.Fraction
			}
			if fraction == nil {
				return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 额度窗口缺少剩余额度: %s", bucket.BucketID)
			}
			if math.IsNaN(*fraction) || math.IsInf(*fraction, 0) || *fraction < 0 || *fraction > 1 {
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
	for _, id := range []string{"gemini-5h", "gemini-weekly"} {
		if _, ok := buckets[id]; !ok {
			return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 刷新结果缺少额度窗口: %s", id)
		}
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
