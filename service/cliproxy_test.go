package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCliproxyAPIClientListAuthFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v0/management/auth-files", r.URL.Path)
		require.Equal(t, "Bearer cliproxyapi", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":[{"auth_index":"f2ca5514ba44085e","name":"codex-duboislee1988@gmail.com-prolite.json","id":"codex-duboislee1988@gmail.com-prolite.json","account_type":"oauth","note":"主账号备注","disabled":false,"id_token":{"chatgpt_account_id":"20ef4492-656e-40a0-8412-af905e51c9f9","plan_type":"prolite"}}]}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	files, err := client.ListAuthFiles(context.Background())
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "f2ca5514ba44085e", files[0].AuthIndex)
	require.Equal(t, "codex-duboislee1988@gmail.com-prolite.json", files[0].Name)
	require.Equal(t, "codex-duboislee1988@gmail.com-prolite.json", files[0].AuthFile)
	require.Equal(t, "20ef4492-656e-40a0-8412-af905e51c9f9", files[0].AccountID)
	require.Equal(t, "prolite", files[0].PlanType)
	require.Equal(t, "主账号备注", files[0].Note)
	require.True(t, files[0].Enabled)
}

func TestNormalizeCliproxyAuthFilesSortsCodexPlans(t *testing.T) {
	files := normalizeCliproxyAuthFiles(cliproxyAuthFilesResponse{
		Files: []cliproxyAuthFileResponse{
			{
				Name:        "free.json",
				AccountType: "oauth",
				IDToken:     cliproxyAuthFileToken{PlanType: "free"},
			},
			{
				Name:        "plus.json",
				AccountType: "oauth",
				IDToken:     cliproxyAuthFileToken{PlanType: "plus"},
			},
			{
				Name:        "prolite.json",
				AccountType: "oauth",
				IDToken:     cliproxyAuthFileToken{PlanType: "prolite"},
			},
			{
				Name:        "pro.json",
				AccountType: "oauth",
				IDToken:     cliproxyAuthFileToken{PlanType: "pro"},
			},
		},
	})

	require.Equal(t, []string{"pro.json", "prolite.json", "plus.json", "free.json"}, []string{
		files[0].Name,
		files[1].Name,
		files[2].Name,
		files[3].Name,
	})
	require.Equal(t, []string{"pro", "prolite", "plus", "free"}, []string{
		files[0].PlanType,
		files[1].PlanType,
		files[2].PlanType,
		files[3].PlanType,
	})
}

func TestNormalizeCliproxyAuthFilesSupportsClaudeMetadata(t *testing.T) {
	files := normalizeCliproxyAuthFiles(cliproxyAuthFilesResponse{
		Files: []cliproxyAuthFileResponse{
			{
				AuthIndex:   "c1fa0ce8add6b367",
				Name:        "claude-hermensdriggars@gmail.com.json",
				ID:          "claude-hermensdriggars@gmail.com.json",
				Account:     "hermensdriggars@gmail.com",
				Email:       "hermensdriggars@gmail.com",
				AccountType: "oauth",
				Provider:    "claude",
				Type:        "claude",
			},
		},
	})

	require.Len(t, files, 1)
	require.Equal(t, "c1fa0ce8add6b367", files[0].AuthIndex)
	require.Equal(t, "claude-hermensdriggars@gmail.com.json", files[0].AuthFile)
	require.Equal(t, "hermensdriggars@gmail.com", files[0].AccountID)
	require.Equal(t, "claude", files[0].PlanType)
	require.Equal(t, "claude", files[0].Provider)
	require.Equal(t, "claude", files[0].Type)
}

func TestNormalizeCliproxyAuthFilesDetectsXAIFromName(t *testing.T) {
	files := normalizeCliproxyAuthFiles(cliproxyAuthFilesResponse{
		Files: []cliproxyAuthFileResponse{
			{
				AuthIndex:   "f30c0c700f97feaf",
				Name:        "xai-gooddgege@gmail.com.json",
				ID:          "xai-gooddgege@gmail.com.json",
				AccountType: "oauth",
			},
		},
	})

	require.Len(t, files, 1)
	require.Equal(t, "xai-gooddgege@gmail.com.json", files[0].AuthFile)
	require.Equal(t, "xai", files[0].PlanType)
	require.Equal(t, "xai", files[0].Provider)
	require.Equal(t, "xai", files[0].Type)
}

func TestCliproxyPlanRankSupportsClaudePlans(t *testing.T) {
	require.Equal(t, cliproxyPlanRank("plus"), cliproxyPlanRank("claude_pro"))
	require.Equal(t, cliproxyPlanRank("prolite"), cliproxyPlanRank("claude_max_5x"))
	require.Equal(t, cliproxyPlanRank("pro"), cliproxyPlanRank("claude_max_20x"))
	require.Less(t, cliproxyPlanRank("claude_max_5x"), cliproxyPlanRank("claude_pro"))
}

func TestCliproxyAPIClientListAuthFilesSupportsDataField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"authIndex":"f2ca5514ba44085e","name":"主账号","accountId":"20ef4492-656e-40a0-8412-af905e51c9f9","enabled":true}]}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	files, err := client.ListAuthFiles(context.Background())
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "f2ca5514ba44085e", files[0].AuthIndex)
	require.Equal(t, "主账号", files[0].Name)
	require.Equal(t, "20ef4492-656e-40a0-8412-af905e51c9f9", files[0].AccountID)
	require.True(t, files[0].Enabled)
}

func TestCliproxyAPIClientCallAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v0/management/api-call", r.URL.Path)
		require.Equal(t, "Bearer cliproxyapi", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"body":{"used_tokens":1234,"quota":5678}}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	result, err := client.CallAPI(context.Background(), CliproxyAPICallRequest{
		AuthIndex: "f2ca5514ba44085e",
		Method:    http.MethodGet,
		URL:       "https://chatgpt.com/backend-api/wham/usage",
		Header: map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"Content-Type":       "application/json",
			"Chatgpt-Account-Id": "20ef4492-656e-40a0-8412-af905e51c9f9",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, result.Status)
	require.Equal(t, float64(1234), result.Body["used_tokens"])
	require.Equal(t, float64(5678), result.Body["quota"])
}

func TestCliproxyAPIClientCallAPISupportsStatusCodeAndStringBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v0/management/api-call", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"body":"{\"plan_type\":\"prolite\",\"rate_limit\":{\"primary_window\":{\"used_percent\":25}}}"}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	result, err := client.CallAPI(context.Background(), CliproxyAPICallRequest{
		AuthIndex: "f2ca5514ba44085e",
		Method:    http.MethodGet,
		URL:       "https://chatgpt.com/backend-api/wham/usage",
	})
	require.NoError(t, err)
	require.Equal(t, 200, result.Status)
	require.Equal(t, "prolite", result.Body["plan_type"])
}

func TestCliproxyAPIClientCallAPIRejectsInnerErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":401,"body":{"error":"token expired"}}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	_, err = client.CallAPI(context.Background(), CliproxyAPICallRequest{
		AuthIndex: "f2ca5514ba44085e",
		Method:    http.MethodGet,
		URL:       "https://chatgpt.com/backend-api/wham/usage",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "上游状态码: 401")
}

func TestCliproxyAPIClientCallAPIPreservesForbiddenDetails(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		status           int
	}{
		{"上游对象错误", `{"status_code":403,"body":{"error":{"status":"PERMISSION_DENIED","message":"Verify your account to continue.","details":[{"reason":"VALIDATION_REQUIRED","metadata":{"validation_url":"https://accounts.google.com/verify?token=private-validation-token"}}]}}}`, "403; PERMISSION_DENIED; VALIDATION_REQUIRED; Verify your account to continue.", http.StatusOK},
		{"上游字符串响应", `{"status_code":403,"body":"{\"error\":{\"message\":\"Verify your account to continue.\",\"details\":[{\"reason\":\"VALIDATION_REQUIRED\"}]}}"}`, "403; VALIDATION_REQUIRED; Verify your account to continue.", http.StatusOK},
		{"管理接口拒绝", `{"error":"management access denied"}`, "403; management access denied", http.StatusForbidden},
		{"错误缺少正文", `{"status_code":403,"body":{}}`, "403", http.StatusOK},
		{"错误包含验证链接", `{"status_code":403,"body":{"error":{"message":"Verify at https://accounts.google.com/verify?token=private-validation-token"}}}`, "Verify at https://accounts.google.com", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)
			client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
			require.NoError(t, err)
			_, err = client.CallAPI(context.Background(), CliproxyAPICallRequest{AuthIndex: "ag", Method: http.MethodPost, URL: "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"})
			require.ErrorContains(t, err, tt.want)
			require.NotContains(t, err.Error(), "private-validation-token")
			require.NotContains(t, err.Error(), "validation_url")
			require.Equal(t, int32(1), hits.Load())
		})
	}
}

func TestNewCliproxyAPIClientRejectsInvalidBaseURL(t *testing.T) {
	_, err := NewCliproxyAPIClient("file:///tmp/socket", "cliproxyapi")
	require.Error(t, err)

	_, err = NewCliproxyAPIClient("https://", "cliproxyapi")
	require.Error(t, err)
}

func TestNewCliproxyAPIClientRejectsEmptyPassword(t *testing.T) {
	_, err := NewCliproxyAPIClient("https://example.com", " ")
	require.Error(t, err)
}

func TestCliproxyAPIClientCallAPIRetriesTransientStatus(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := hits.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"body":{"ok":true}}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	result, err := client.CallAPI(context.Background(), CliproxyAPICallRequest{
		AuthIndex: "xai-auth",
		Method:    http.MethodGet,
		URL:       "https://cli-chat-proxy.grok.com/v1/billing",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int32(3), hits.Load())
	require.Equal(t, true, result.Body["ok"])
}

func TestCliproxyAPIClientCallAPIDoesNotRetryClientErrors(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	client, err := NewCliproxyAPIClient(server.URL, "cliproxyapi")
	require.NoError(t, err)

	_, err = client.CallAPI(context.Background(), CliproxyAPICallRequest{
		AuthIndex: "xai-auth",
		Method:    http.MethodGet,
		URL:       "https://cli-chat-proxy.grok.com/v1/billing",
	})
	require.Error(t, err)
	require.Equal(t, int32(1), hits.Load())
	require.Contains(t, err.Error(), "403")
}
