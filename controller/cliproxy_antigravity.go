package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
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

func refreshCliproxyAntigravityUsage(ctx context.Context, caller cliproxyAPICaller, binding *model.CliproxyAuthFileBinding) (cliproxyUsageRefreshBody, error) {
	request := buildCliproxyUsageRefreshRequest(binding)
	var lastErr error
	for _, method := range []string{"retrieveUserQuotaSummary", "fetchAvailableModels"} {
		for _, host := range []string{"daily-cloudcode-pa.googleapis.com", "daily-cloudcode-pa.sandbox.googleapis.com", "cloudcode-pa.googleapis.com"} {
			if ctx.Err() != nil {
				return cliproxyUsageRefreshBody{}, ctx.Err()
			}
			request.URL = "https://" + host + "/v1internal:" + method
			result, err := caller.CallAPI(ctx, request)
			if err != nil {
				var statusErr interface{ StatusCode() int }
				if errors.As(err, &statusErr) && statusErr.StatusCode() == http.StatusTooManyRequests {
					return cliproxyUsageRefreshBody{}, err
				}
				lastErr = err
				continue
			}
			if result == nil {
				lastErr = fmt.Errorf("Antigravity 刷新结果为空")
				continue
			}
			status := max(result.Status, result.StatusCode)
			if status >= http.StatusBadRequest {
				lastErr = fmt.Errorf("刷新额度失败，上游状态码: %d", status)
				if status == http.StatusTooManyRequests {
					return cliproxyUsageRefreshBody{}, lastErr
				}
				continue
			}
			body := result.Body
			if len(body) == 0 {
				body = result.Data
			}
			usage, err := extractCliproxyAntigravityUsage(body)
			if err == nil {
				return usage, nil
			}
			lastErr = err
		}
	}
	return cliproxyUsageRefreshBody{}, lastErr
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
		Models map[string]struct {
			DisplayName          string `json:"displayName"`
			DisplayNameSnakeCase string `json:"display_name"`
			ModelProvider        string `json:"modelProvider"`
			QuotaInfo            *struct {
				RemainingFraction *float64 `json:"remainingFraction"`
				ResetTime         string   `json:"resetTime"`
			} `json:"quotaInfo"`
		} `json:"models"`
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
		// 模型额度没有五小时或每周窗口语义，按提供商保留最少的剩余额度。
		for id, item := range response.Models {
			if item.QuotaInfo == nil || item.QuotaInfo.RemainingFraction == nil {
				continue
			}
			name := strings.ToLower(id + " " + item.DisplayName + " " + item.DisplayNameSnakeCase + " " + item.ModelProvider)
			var bucketID string
			switch {
			case strings.Contains(name, "claude"), strings.Contains(name, "gpt"), strings.Contains(name, "anthropic"), strings.Contains(name, "openai"):
				bucketID = "3p-shared"
			case strings.Contains(name, "gemini"):
				bucketID = "gemini-shared"
			default:
				continue
			}
			fraction := item.QuotaInfo.RemainingFraction
			if math.IsNaN(*fraction) || math.IsInf(*fraction, 0) || *fraction < 0 || *fraction > 1 {
				return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 剩余额度超出范围: %s", id)
			}
			var resetAt int64
			if item.QuotaInfo.ResetTime != "" {
				reset, err := time.Parse(time.RFC3339, item.QuotaInfo.ResetTime)
				if err != nil {
					return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 重置时间无效: %s", id)
				}
				resetAt = reset.Unix()
			}
			bucket, exists := buckets[bucketID]
			if !exists || *fraction < *bucket.RemainingFraction {
				buckets[bucketID] = cliproxyAntigravityQuotaBucket{BucketID: bucketID, RemainingFraction: fraction, ResetAt: resetAt}
			} else if *fraction == *bucket.RemainingFraction {
				// 限制额度的模型中任一重置时间未知时，不推断整个分组的恢复时间。
				if resetAt == 0 || bucket.ResetAt == 0 {
					bucket.ResetAt = 0
				} else {
					bucket.ResetAt = max(bucket.ResetAt, resetAt)
				}
				buckets[bucketID] = bucket
			}
		}
	}
	if len(buckets) == 0 {
		return cliproxyUsageRefreshBody{}, fmt.Errorf("Antigravity 刷新结果缺少额度窗口或模型额度")
	}
	ordered := make([]cliproxyAntigravityQuotaBucket, 0, len(buckets))
	for _, id := range []string{"gemini-5h", "gemini-weekly", "3p-5h", "3p-weekly", "gemini-shared", "3p-shared"} {
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
