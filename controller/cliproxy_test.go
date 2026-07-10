package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

type fakeCliproxyAPICaller struct {
	responses map[string]*service.CliproxyAPICallResponse
	errors    map[string]error
}

func (f *fakeCliproxyAPICaller) CallAPI(_ context.Context, request service.CliproxyAPICallRequest) (*service.CliproxyAPICallResponse, error) {
	return f.responses[request.URL], f.errors[request.URL]
}

func TestExtractCliproxyUsageSupportsClaudeUsagePayload(t *testing.T) {
	result := &service.CliproxyAPICallResponse{
		Body: map[string]any{
			"five_hour": map[string]any{
				"utilization": float64(25),
				"resets_at":   "2026-06-11T10:00:00Z",
			},
			"seven_day": map[string]any{
				"utilization": "10",
				"resets_at":   "2026-06-18T10:00:00Z",
			},
		},
	}

	usage, err := extractCliproxyUsage(result)
	require.NoError(t, err)
	require.Equal(t, "claude", usage.PlanType)
	require.Equal(t, 25, usage.FiveHourPercent)
	require.Equal(t, time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC).Unix(), usage.FiveHourResetAt)
	require.Equal(t, 10, usage.WeeklyPercent)
	require.Equal(t, time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC).Unix(), usage.WeeklyResetAt)
}

func TestBuildCliproxyUsageRefreshRequestUsesClaudeOAuthUsage(t *testing.T) {
	request := buildCliproxyUsageRefreshRequest(&model.CliproxyAuthFileBinding{
		AuthIndex: "c1fa0ce8add6b367",
		AuthFile:  "claude-hermensdriggars@gmail.com.json",
		AuthName:  "claude-hermensdriggars@gmail.com.json",
	})

	require.Equal(t, "c1fa0ce8add6b367", request.AuthIndex)
	require.Equal(t, cliproxyClaudeUsageURL, request.URL)
	require.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
	require.Equal(t, "oauth-2025-04-20", request.Header["anthropic-beta"])
	require.NotContains(t, request.Header, "Chatgpt-Account-Id")
}

func TestBuildCliproxyUsageRefreshRequestUsesXAIBilling(t *testing.T) {
	request := buildCliproxyUsageRefreshRequest(&model.CliproxyAuthFileBinding{
		AuthIndex: "f30c0c700f97feaf",
		AuthFile:  "xai-gooddgege@gmail.com.json",
		AuthName:  "xai-gooddgege@gmail.com.json",
	})

	require.Equal(t, "f30c0c700f97feaf", request.AuthIndex)
	require.Equal(t, cliproxyXAIBillingURL, request.URL)
	require.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
	require.NotContains(t, request.Header, "Chatgpt-Account-Id")
}

func TestExtractCliproxyUsageSupportsXAIBillingPayload(t *testing.T) {
	result := &service.CliproxyAPICallResponse{
		Body: map[string]any{
			"config": map[string]any{
				"monthlyLimit":     map[string]any{"val": float64(15000)},
				"used":             map[string]any{"val": float64(123)},
				"onDemandCap":      map[string]any{"val": float64(2500)},
				"billingPeriodEnd": "2026-08-01T00:00:00+00:00",
			},
		},
	}

	usage, err := extractCliproxyUsage(result)

	require.NoError(t, err)
	require.Equal(t, "SuperGrok", usage.PlanType)
	require.Equal(t, 123, usage.UsedTokens)
	require.Equal(t, 15000, usage.Quota)
	require.Equal(t, 2500, usage.OnDemandCap)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Unix(), usage.BillingPeriodEndAt)
}

func TestResolveCliproxyXAIWeeklyUsage(t *testing.T) {
	usage, ok := resolveCliproxyXAIWeeklyUsage(map[string]any{
		"config": map[string]any{
			"currentPeriod": map[string]any{
				"type":  "weekly",
				"start": "2026-07-09T13:16:00Z",
				"end":   "2026-07-16T13:16:00Z",
			},
			"creditUsagePercent": float64(45),
			"productUsage": []any{
				map[string]any{"product": "Api", "usagePercent": float64(45)},
			},
		},
	})

	require.True(t, ok)
	require.Equal(t, 45, usage.XAIWeeklyPercent)
	require.Equal(t, time.Date(2026, 7, 9, 13, 16, 0, 0, time.UTC).Unix(), usage.XAIWeeklyPeriodStartAt)
	require.Equal(t, time.Date(2026, 7, 16, 13, 16, 0, 0, time.UTC).Unix(), usage.XAIWeeklyPeriodEndAt)
	require.JSONEq(t, `[{"product":"Api","usage_percent":45}]`, usage.XAIProductUsage)
}

func TestResolveCliproxyXAIMonthlyUsageSupportsSnakeCaseAndHeavyPlan(t *testing.T) {
	usage, ok := resolveCliproxyXAIMonthlyUsage(map[string]any{
		"config": map[string]any{
			"monthly_limit":      map[string]any{"val": float64(150000)},
			"used":               map[string]any{"val": float64(42000)},
			"on_demand_cap":      map[string]any{"val": float64(10000)},
			"on_demand_used":     map[string]any{"val": float64(300)},
			"billing_period_end": "2026-08-01T00:00:00Z",
		},
	})

	require.True(t, ok)
	require.Equal(t, "SuperGrok Heavy", usage.PlanType)
	require.Equal(t, 150000, usage.Quota)
	require.Equal(t, 42000, usage.UsedTokens)
	require.Equal(t, 10000, usage.OnDemandCap)
	require.Equal(t, 300, usage.XAIOnDemandUsed)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Unix(), usage.BillingPeriodEndAt)
}

func TestBuildCliproxyXAIBillingRequestsUseGrokCLIHeaders(t *testing.T) {
	weeklyRequest := buildCliproxyXAIWeeklyBillingRequest("auth-index")
	monthlyRequest := buildCliproxyXAIMonthlyBillingRequest("auth-index")

	require.Equal(t, cliproxyXAIWeeklyBillingURL, weeklyRequest.URL)
	require.Equal(t, cliproxyXAIMonthlyBillingURL, monthlyRequest.URL)
	for _, request := range []service.CliproxyAPICallRequest{weeklyRequest, monthlyRequest} {
		require.Equal(t, "auth-index", request.AuthIndex)
		require.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
		require.Equal(t, "xai-grok-cli", request.Header["x-xai-token-auth"])
		require.Equal(t, "0.2.91", request.Header["x-grok-client-version"])
		require.Equal(t, "*/*", request.Header["accept"])
		require.Contains(t, request.Header["user-agent"], "grok-shell/0.2.91")
	}
}

func TestResolveCliproxyClaudeProfilePlan(t *testing.T) {
	require.Equal(t, "plan_max", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account": map[string]any{"has_claude_max": true},
	}))
	require.Equal(t, "plus", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account": map[string]any{"has_claude_pro": true},
	}))
	require.Equal(t, "plan_team", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account":      map[string]any{"has_claude_max": false, "has_claude_pro": false},
		"organization": map[string]any{"organization_type": "claude_team", "subscription_status": "active"},
	}))
	require.Equal(t, "plan_free", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account": map[string]any{"has_claude_max": false, "has_claude_pro": false},
	}))
}

func TestRefreshCliproxyXAIUsageMergesWeeklyAndMonthlySnapshots(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{
			cliproxyXAIWeeklyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"currentPeriod":      map[string]any{"start": "2026-07-09T13:16:00Z", "end": "2026-07-16T13:16:00Z"},
					"creditUsagePercent": float64(45),
					"productUsage":       []any{map[string]any{"product": "Api", "usagePercent": float64(45)}},
				}},
			},
			cliproxyXAIMonthlyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"monthlyLimit":     map[string]any{"val": float64(15000)},
					"used":             map[string]any{"val": float64(1768)},
					"onDemandCap":      map[string]any{"val": float64(2500)},
					"onDemandUsed":     map[string]any{"val": float64(125)},
					"billingPeriodEnd": "2026-08-01T00:00:00Z",
				}},
			},
		},
		errors: map[string]error{},
	}

	result, err := refreshCliproxyXAIUsage(context.Background(), caller, &model.CliproxyAuthFileBinding{AuthIndex: "xai-auth"})

	require.NoError(t, err)
	require.Empty(t, result.Warning)
	require.False(t, result.AllowPartialUsage)
	require.Equal(t, 45, result.Usage.XAIWeeklyPercent)
	require.Equal(t, 1768, result.Usage.UsedTokens)
	require.Equal(t, 15000, result.Usage.Quota)
	require.Equal(t, 2500, result.Usage.OnDemandCap)
	require.Equal(t, 125, result.Usage.XAIOnDemandUsed)
}

func TestRefreshCliproxyXAIUsagePreservesMonthlySnapshotWhenMonthlyRefreshFails(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{
			cliproxyXAIWeeklyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"currentPeriod":      map[string]any{"start": "2026-07-09T13:16:00Z", "end": "2026-07-16T13:16:00Z"},
					"creditUsagePercent": float64(20),
				}},
			},
		},
		errors: map[string]error{cliproxyXAIMonthlyBillingURL: errors.New("timeout")},
	}
	binding := &model.CliproxyAuthFileBinding{
		AuthIndex:                 "xai-auth",
		LastUsageTokens:           1768,
		LastUsageQuota:            15000,
		LastPlanType:              "SuperGrok",
		LastXAIOnDemandCap:        2500,
		LastXAIOnDemandUsed:       125,
		LastXAIBillingPeriodEndAt: 1785542400,
	}

	result, err := refreshCliproxyXAIUsage(context.Background(), caller, binding)

	require.NoError(t, err)
	require.True(t, result.AllowPartialUsage)
	require.Contains(t, result.Warning, "月度额度刷新失败")
	require.Equal(t, 20, result.Usage.XAIWeeklyPercent)
	require.Equal(t, 1768, result.Usage.UsedTokens)
	require.Equal(t, 15000, result.Usage.Quota)
	require.Equal(t, 2500, result.Usage.OnDemandCap)
	require.Equal(t, 125, result.Usage.XAIOnDemandUsed)
}

func TestRefreshCliproxyXAIUsagePreservesWeeklySnapshotWhenWeeklyRefreshFails(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{
			cliproxyXAIMonthlyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"monthlyLimit": map[string]any{"val": float64(15000)},
					"used":         map[string]any{"val": float64(5000)},
				}},
			},
		},
		errors: map[string]error{cliproxyXAIWeeklyBillingURL: errors.New("forbidden")},
	}
	binding := &model.CliproxyAuthFileBinding{
		AuthIndex:                  "xai-auth",
		LastXAIWeeklyPercent:       45,
		LastXAIWeeklyPeriodStartAt: 1783599360,
		LastXAIWeeklyPeriodEndAt:   1784204160,
		LastXAIProductUsage:        `[{"product":"Api","usage_percent":45}]`,
	}

	result, err := refreshCliproxyXAIUsage(context.Background(), caller, binding)

	require.NoError(t, err)
	require.True(t, result.AllowPartialUsage)
	require.Contains(t, result.Warning, "周额度刷新失败")
	require.Equal(t, 45, result.Usage.XAIWeeklyPercent)
	require.Equal(t, int64(1783599360), result.Usage.XAIWeeklyPeriodStartAt)
	require.JSONEq(t, `[{"product":"Api","usage_percent":45}]`, result.Usage.XAIProductUsage)
	require.Equal(t, 5000, result.Usage.UsedTokens)
}

func TestRefreshCliproxyXAIUsageReturnsErrorWhenBothRequestsFail(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{},
		errors: map[string]error{
			cliproxyXAIWeeklyBillingURL:  errors.New("weekly failed"),
			cliproxyXAIMonthlyBillingURL: errors.New("monthly failed"),
		},
	}

	_, err := refreshCliproxyXAIUsage(context.Background(), caller, &model.CliproxyAuthFileBinding{AuthIndex: "xai-auth"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "weekly failed")
	require.Contains(t, err.Error(), "monthly failed")
}
