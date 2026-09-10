package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCliproxyAuthFilesRecognizesAntigravity(t *testing.T) {
	for _, input := range []cliproxyAuthFileResponse{
		{Name: "antigravity-test@example.com.json", AccountType: "oauth"},
		{Name: "account.json", Provider: "antigravity", Type: "antigravity", AccountType: "oauth"},
	} {
		files := normalizeCliproxyAuthFiles(cliproxyAuthFilesResponse{Files: []cliproxyAuthFileResponse{input}})
		require.Len(t, files, 1)
		assert.Equal(t, "antigravity", files[0].Provider)
		assert.Equal(t, "antigravity", files[0].PlanType)
	}
}

func TestCliproxyCallAPIForwardsAntigravityProjectBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v0/management/api-call", r.URL.Path)
		var request CliproxyAPICallRequest
		if err := common.DecodeJson(r.Body, &request); !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, "ag", request.AuthIndex)
		assert.Equal(t, "POST", request.Method)
		assert.JSONEq(t, `{"project":"aicode-consumers"}`, request.Data)
		_, err := w.Write([]byte(`{"status_code":200,"body":"{\"groups\":[{\"buckets\":[{\"bucketId\":\"gemini-5h\",\"remainingFraction\":1}]}]}"}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)
	client, err := NewCliproxyAPIClient(upstream.URL, "test-password")
	require.NoError(t, err)
	result, err := client.CallAPI(context.Background(), CliproxyAPICallRequest{AuthIndex: "ag", Method: http.MethodPost, URL: "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", Data: `{"project":"aicode-consumers"}`})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Status)
	assert.Contains(t, result.Body, "groups")
}
