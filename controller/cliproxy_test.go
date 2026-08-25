package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
			"limits": []any{
				map[string]any{
					"kind":      "weekly_scoped",
					"group":     "weekly",
					"percent":   float64(72),
					"resets_at": "2026-06-19T10:00:00Z",
					"scope": map[string]any{
						"model": map[string]any{"display_name": "Fable 5 Max"},
					},
					"is_active": true,
				},
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
	require.Equal(t, 72, usage.ClaudeFablePercent)
	require.Equal(t, time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC).Unix(), usage.ClaudeFableResetAt)
}

func TestExtractCliproxyUsageSupportsClaudeLimitsOnlyPayload(t *testing.T) {
	result := &service.CliproxyAPICallResponse{
		Body: map[string]any{
			"limits": []any{
				map[string]any{
					"kind":      "session",
					"group":     "session",
					"percent":   "18",
					"resets_at": "2026-07-01T10:00:00Z",
					"is_active": true,
				},
				map[string]any{
					"kind":     "weekly_all",
					"group":    "weekly",
					"percent":  float64(34),
					"resetsAt": float64(1783418400000),
					"isActive": true,
				},
				map[string]any{
					"kind":     "weekly_scoped",
					"group":    "weekly",
					"percent":  float64(56),
					"reset_at": "2026-07-08T10:00:00Z",
					"scope": map[string]any{
						"model": map[string]any{"displayName": "Fable 5 Max"},
					},
				},
			},
		},
	}

	usage, err := extractCliproxyUsage(result)
	require.NoError(t, err)
	require.Equal(t, 18, usage.FiveHourPercent)
	require.Equal(t, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC).Unix(), usage.FiveHourResetAt)
	require.Equal(t, 34, usage.WeeklyPercent)
	require.Equal(t, int64(1783418400), usage.WeeklyResetAt)
	require.Equal(t, 56, usage.ClaudeFablePercent)
	require.Equal(t, time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC).Unix(), usage.ClaudeFableResetAt)
}

func TestExtractCliproxyUsageSupportsCodexWhamPayload(t *testing.T) {
	result := &service.CliproxyAPICallResponse{
		Body: map[string]any{
			"plan_type": "pro",
			"rate_limit": map[string]any{
				"primary_window": map[string]any{
					"used_percent":         float64(41),
					"limit_window_seconds": float64(604800),
					"reset_at":             float64(1788137789),
				},
			},
			"additional_rate_limits": []any{
				map[string]any{
					"limit_name":      "GPT-5.3-Codex-Spark",
					"metered_feature": "codex_bengalfox",
					"rate_limit": map[string]any{
						"primary_window": map[string]any{
							"used_percent":         float64(11),
							"limit_window_seconds": float64(18000),
							"reset_at":             float64(1787663429),
						},
						"secondary_window": map[string]any{
							"used_percent":         float64(12),
							"limit_window_seconds": float64(604800),
							"reset_at":             float64(1788164344),
						},
					},
				},
			},
		},
	}

	usage, err := extractCliproxyUsage(result)

	require.NoError(t, err)
	require.Equal(t, "pro", usage.PlanType)
	require.Zero(t, usage.FiveHourPercent)
	require.Equal(t, 41, usage.WeeklyPercent)
	require.Equal(t, int64(1788137789), usage.WeeklyResetAt)
	require.Equal(t, 11, usage.CodexFiveHourPercent)
	require.Equal(t, int64(1787663429), usage.CodexFiveHourResetAt)
	require.Equal(t, 12, usage.CodexWeeklyPercent)
	require.Equal(t, int64(1788164344), usage.CodexWeeklyResetAt)
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

func TestIsCliproxyXAIAuthFileRecognizesCurrentPlans(t *testing.T) {
	for _, plan := range []string{
		"SuperGrok Lite",
		"SuperGrok",
		"SuperGrok Plus",
		"SuperGrok Heavy",
		"SUBSCRIPTION_TIER_SUPER_GROK_PLUS",
	} {
		require.True(t, isCliproxyXAIAuthFile(&model.CliproxyAuthFileBinding{
			AuthName:     "account@example.com.json",
			LastPlanType: plan,
		}), plan)
	}
}

func TestExtractCliproxyUsageSupportsXAIBillingPayload(t *testing.T) {
	result := &service.CliproxyAPICallResponse{
		Body: map[string]any{
			"config": map[string]any{
				"monthlyLimit":     map[string]any{"val": float64(15000)},
				"used":             map[string]any{"val": float64(123)},
				"onDemandCap":      map[string]any{"val": float64(2500)},
				"subscriptionTier": "SUBSCRIPTION_TIER_SUPER_GROK",
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
			"subscription_tier":  "SUBSCRIPTION_TIER_SUPER_GROK_HEAVY",
			"billing_period_end": "2026-08-01T00:00:00Z",
		},
	})

	require.True(t, ok)
	require.Equal(t, "SuperGrok Heavy", usage.PlanType)
	require.Equal(t, 150000, usage.Quota)
	require.Equal(t, 42000, usage.UsedTokens)
	require.Equal(t, 10000, usage.OnDemandCap)
	require.Equal(t, 300, usage.XAIOnDemandUsed)
	require.True(t, usage.XAIOnDemandUsedRefreshed)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Unix(), usage.BillingPeriodEndAt)
}

func TestResolveCliproxyXAIMonthlyUsageSupportsBillingCycleUsagePayload(t *testing.T) {
	usage, ok := resolveCliproxyXAIMonthlyUsage(map[string]any{
		"billingCycle": map[string]any{
			"billingPeriodStart": "2026-07-01T00:00:00Z",
			"billingPeriodEnd":   "2026-08-01T00:00:00Z",
		},
		"monthlyLimit":            map[string]any{"val": float64(150000)},
		"onDemandCap":             map[string]any{"val": float64(0)},
		"subscriptionTierDisplay": "SuperGrok Plus",
		"usage": map[string]any{
			"includedUsed": map[string]any{"val": float64(9383)},
			"onDemandUsed": map[string]any{"val": float64(0)},
			"totalUsed":    map[string]any{"val": float64(9383)},
		},
	})

	require.True(t, ok)
	require.Equal(t, "SuperGrok Plus", usage.PlanType)
	require.Equal(t, 150000, usage.Quota)
	require.Equal(t, 9383, usage.UsedTokens)
	require.Zero(t, usage.OnDemandCap)
	require.Zero(t, usage.XAIOnDemandUsed)
	require.True(t, usage.XAIOnDemandUsedRefreshed)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Unix(), usage.BillingPeriodEndAt)
}

func TestResolveCliproxyXAIMonthlyUsageDoesNotInferPlanFromQuota(t *testing.T) {
	usage, ok := resolveCliproxyXAIMonthlyUsage(map[string]any{
		"monthlyLimit": map[string]any{"val": float64(150000)},
		"used":         map[string]any{"val": float64(100)},
	})

	require.True(t, ok)
	require.Empty(t, usage.PlanType)
}

func TestExtractCliproxyXAIPlanUsesSettingsDisplayTier(t *testing.T) {
	plan := extractCliproxyXAIPlan(&service.CliproxyAPICallResponse{
		Body: map[string]any{
			"settings": map[string]any{"subscription_tier_display": "SuperGrok Lite"},
		},
	})

	require.Equal(t, "SuperGrok Lite", plan)
}

func TestResolveCliproxyXAIMonthlyUsageMarksExplicitZeroOnDemandUsageAsRefreshed(t *testing.T) {
	usage, ok := resolveCliproxyXAIMonthlyUsage(map[string]any{
		"config": map[string]any{
			"monthlyLimit": map[string]any{"val": float64(15000)},
			"used":         map[string]any{"val": float64(17000)},
			"onDemandCap":  map[string]any{"val": float64(5000)},
			"onDemandUsed": map[string]any{"val": float64(0)},
		},
	})

	require.True(t, ok)
	require.Zero(t, usage.XAIOnDemandUsed)
	require.True(t, usage.XAIOnDemandUsedRefreshed)
}

func TestBuildCliproxyXAIBillingRequestsUseGrokCLIHeaders(t *testing.T) {
	weeklyRequest := buildCliproxyXAIWeeklyBillingRequest("auth-index")
	monthlyRequest := buildCliproxyXAIMonthlyBillingRequest("auth-index")
	settingsRequest := buildCliproxyXAISettingsRequest("auth-index")

	require.Equal(t, cliproxyXAIWeeklyBillingURL, weeklyRequest.URL)
	require.Equal(t, cliproxyXAIMonthlyBillingURL, monthlyRequest.URL)
	require.Equal(t, cliproxyXAISettingsURL, settingsRequest.URL)
	for _, request := range []service.CliproxyAPICallRequest{weeklyRequest, monthlyRequest, settingsRequest} {
		require.Equal(t, "auth-index", request.AuthIndex)
		require.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
		require.Equal(t, "xai-grok-cli", request.Header["x-xai-token-auth"])
		require.Equal(t, "0.2.91", request.Header["x-grok-client-version"])
		require.Equal(t, "*/*", request.Header["accept"])
		require.Contains(t, request.Header["user-agent"], "grok-shell/0.2.91")
	}
}

func TestResolveCliproxyClaudeProfilePlan(t *testing.T) {
	tests := []struct {
		name     string
		profile  map[string]any
		expected string
	}{
		{
			name: "pro organization",
			profile: map[string]any{
				"organization": map[string]any{"organization_type": "claude_pro"},
			},
			expected: "claude_pro",
		},
		{
			name: "max 5x rate limit tier",
			profile: map[string]any{
				"organization": map[string]any{
					"organization_type": "claude_max",
					"rate_limit_tier":   "default_claude_max_5x",
				},
			},
			expected: "claude_max_5x",
		},
		{
			name: "max 20x rate limit tier",
			profile: map[string]any{
				"organization": map[string]any{
					"organization_type": "claude_max",
					"rate_limit_tier":   "default_claude_max_20x",
				},
			},
			expected: "claude_max_20x",
		},
		{
			name: "max without rate limit tier",
			profile: map[string]any{
				"account":      map[string]any{"has_claude_max": true},
				"organization": map[string]any{"organization_type": "claude_max"},
			},
			expected: "claude_max",
		},
		{
			name:     "legacy pro boolean",
			profile:  map[string]any{"account": map[string]any{"has_claude_pro": true}},
			expected: "claude_pro",
		},
		{
			name: "team organization",
			profile: map[string]any{
				"account":      map[string]any{"has_claude_max": false, "has_claude_pro": false},
				"organization": map[string]any{"organization_type": "claude_team", "subscription_status": "active"},
			},
			expected: "claude_team",
		},
		{
			name:     "free account",
			profile:  map[string]any{"account": map[string]any{"has_claude_max": false, "has_claude_pro": false}},
			expected: "claude_free",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, resolveCliproxyClaudeProfilePlan(test.profile))
		})
	}
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
			cliproxyXAISettingsURL: {
				Body: map[string]any{"subscription_tier_display": "SuperGrok Plus"},
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
	require.Equal(t, "SuperGrok Plus", result.Usage.PlanType)
}

func TestRefreshCliproxyXAIUsageClearsGuessedPlanWhenSettingsIsUnavailable(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{
			cliproxyXAIWeeklyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"currentPeriod":      map[string]any{"start": "2026-07-09T13:16:00Z", "end": "2026-07-16T13:16:00Z"},
					"creditUsagePercent": float64(45),
				}},
			},
			cliproxyXAIMonthlyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"monthlyLimit": map[string]any{"val": float64(150000)},
					"used":         map[string]any{"val": float64(1768)},
				}},
			},
		},
		errors: map[string]error{cliproxyXAISettingsURL: errors.New("unauthorized")},
	}
	binding := &model.CliproxyAuthFileBinding{
		AuthIndex:    "xai-auth",
		LastPlanType: "SuperGrok Heavy",
	}

	result, err := refreshCliproxyXAIUsage(context.Background(), caller, binding)

	require.NoError(t, err)
	require.Empty(t, result.Usage.PlanType)
}

func TestRefreshCliproxyXAIUsageFallsBackToMonthlyUsedPercentWhenWeeklyResponseOmitsPercent(t *testing.T) {
	caller := &fakeCliproxyAPICaller{
		responses: map[string]*service.CliproxyAPICallResponse{
			cliproxyXAIWeeklyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"currentPeriod": map[string]any{
						"type":  "USAGE_PERIOD_TYPE_WEEKLY",
						"start": "2026-07-09T13:16:07.977968+00:00",
						"end":   "2026-07-16T13:16:07.977968+00:00",
					},
					"onDemandCap":        map[string]any{"val": float64(0)},
					"onDemandUsed":       map[string]any{"val": float64(0)},
					"billingPeriodStart": "2026-07-09T13:16:07.977968+00:00",
					"billingPeriodEnd":   "2026-07-16T13:16:07.977968+00:00",
				}},
			},
			cliproxyXAIMonthlyBillingURL: {
				Body: map[string]any{"config": map[string]any{
					"monthlyLimit":       map[string]any{"val": float64(15000)},
					"used":               map[string]any{"val": float64(6134)},
					"onDemandCap":        map[string]any{"val": float64(0)},
					"billingPeriodStart": "2026-07-01T00:00:00+00:00",
					"billingPeriodEnd":   "2026-08-01T00:00:00+00:00",
				}},
			},
		},
		errors: map[string]error{},
	}

	result, err := refreshCliproxyXAIUsage(context.Background(), caller, &model.CliproxyAuthFileBinding{AuthIndex: "xai-auth"})

	require.NoError(t, err)
	require.Equal(t, 41, result.Usage.XAIWeeklyPercent)
	require.Equal(t, time.Date(2026, 7, 9, 13, 16, 7, 0, time.UTC).Unix(), result.Usage.XAIWeeklyPeriodStartAt)
	require.Equal(t, time.Date(2026, 7, 16, 13, 16, 7, 0, time.UTC).Unix(), result.Usage.XAIWeeklyPeriodEndAt)
	require.Equal(t, 6134, result.Usage.UsedTokens)
	require.Equal(t, 15000, result.Usage.Quota)
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

func TestRefreshCliproxyAuthFileBindingUsageReturnsFailureAndPreservesXAIRefreshTimeWhenBothRequestsFail(t *testing.T) {
	originalDB := model.DB
	t.Cleanup(func() {
		model.DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CliproxyAuthFileBinding{}))
	model.DB = db

	const lastRefreshedAt int64 = 1784204160
	require.NoError(t, db.Create(&model.CliproxyAuthFileBinding{
		Id:                         1,
		UserId:                     1,
		Username:                   "root",
		AuthIndex:                  "xai-auth",
		AuthName:                   "xai-root@example.com.json",
		Enabled:                    true,
		LastRefreshedAt:            lastRefreshedAt,
		LastUsageTokens:            1768,
		LastUsageQuota:             15000,
		LastPlanType:               "SuperGrok",
		LastXAIWeeklyPercent:       45,
		LastXAIWeeklyPeriodStartAt: 1783599360,
		LastXAIWeeklyPeriodEndAt:   1784204160,
		LastXAIProductUsage:        `[{"product":"Api","usage_percent":45}]`,
		LastXAIOnDemandCap:         2500,
		LastXAIOnDemandUsed:        300,
		LastXAIBillingPeriodEndAt:  1785542400,
	}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		"CliproxyAPIBaseURL":  upstream.URL,
		"CliproxyAPIPassword": "test-password",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	router := gin.New()
	router.POST("/api/cliproxy/auth-files/bindings/:id/refresh-usage", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", common.RoleAdminUser)
		RefreshCliproxyAuthFileBindingUsage(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/cliproxy/auth-files/bindings/1/refresh-usage", nil))

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "周额度刷新失败")
	assert.Contains(t, response.Message, "月度额度刷新失败")

	stored, err := model.GetCliproxyAuthFileBindingById(1)
	require.NoError(t, err)
	assert.Equal(t, lastRefreshedAt, stored.LastRefreshedAt)
	assert.Equal(t, 1768, stored.LastUsageTokens)
	assert.Equal(t, 15000, stored.LastUsageQuota)
	assert.Equal(t, 45, stored.LastXAIWeeklyPercent)
	assert.Equal(t, 2500, stored.LastXAIOnDemandCap)
	assert.Equal(t, 300, stored.LastXAIOnDemandUsed)
	assert.Contains(t, stored.LastError, "周额度刷新失败")
	assert.Contains(t, stored.LastError, "月度额度刷新失败")
}
