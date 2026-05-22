package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCliproxyAPIClientListAuthFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v0/management/auth-files", r.URL.Path)
		require.Equal(t, "Bearer cliproxyapi", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":[{"auth_index":"f2ca5514ba44085e","name":"codex-duboislee1988@gmail.com-prolite.json","id":"codex-duboislee1988@gmail.com-prolite.json","disabled":false,"id_token":{"chatgpt_account_id":"20ef4492-656e-40a0-8412-af905e51c9f9"}}]}`))
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
	require.True(t, files[0].Enabled)
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
