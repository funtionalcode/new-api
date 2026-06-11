package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

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

func TestResolveCliproxyClaudeProfilePlan(t *testing.T) {
	require.Equal(t, "plan_max", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account": map[string]any{"has_claude_max": true},
	}))
	require.Equal(t, "plan_team", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account":      map[string]any{"has_claude_max": false, "has_claude_pro": false},
		"organization": map[string]any{"organization_type": "claude_team", "subscription_status": "active"},
	}))
	require.Equal(t, "plan_free", resolveCliproxyClaudeProfilePlan(map[string]any{
		"account": map[string]any{"has_claude_max": false, "has_claude_pro": false},
	}))
}
