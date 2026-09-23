package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const typeSafeTestCurl = "curl 'https://console.typesafe.ai/api/usage?granularity=day' -b 'session=test-only' -H 'Host: attacker.invalid'"

func TestTypeSafeUsageRequestRestrictsCredentialDestination(t *testing.T) {
	config, err := buildTypeSafeUsageRequest(typeSafeTestCurl)
	require.NoError(t, err)
	assert.Equal(t, "https://console.typesafe.ai/api/usage?granularity=hour", config.URL)
	assert.Equal(t, map[string]string{"Cookie": "session=test-only"}, config.Headers)
	for _, raw := range []string{
		strings.Replace(typeSafeTestCurl, "https:", "http:", 1),
		strings.Replace(typeSafeTestCurl, "console.typesafe.ai", "attacker.invalid", 1),
		strings.Replace(typeSafeTestCurl, "/api/usage", "/api/api-keys", 1),
		strings.Replace(typeSafeTestCurl, "console.typesafe.ai", "console.typesafe.ai:443", 1),
		typeSafeTestCurl + " -X POST",
		typeSafeTestCurl + " --data '{}'",
		"curl https://console.typesafe.ai/api/usage",
	} {
		_, err := buildTypeSafeUsageRequest(raw)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "test-only")
	}
}

func TestTypeSafeUsageResponseValidation(t *testing.T) {
	for _, body := range []string{`{}`, `{"buckets":null}`, `<html>Login</html>`,
		`{"buckets":[{"day":"invalid"}]}`,
		`{"buckets":[{"day":"2026-09-23T00:00:00Z","inputTokens":-1}]}`,
		`{"buckets":[{"day":"2026-09-23T00:00:00Z","requests":9007199254740992}]}`,
	} {
		_, err := decodeTypeSafeUsage([]byte(body))
		require.Error(t, err)
	}
	encoded, err := decodeTypeSafeUsage([]byte(`{"buckets":[]}`))
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, encoded)
	encoded, err = decodeTypeSafeUsage([]byte(`{"buckets":[{"day":"2026-09-23T00:00:00Z","requests":2,"inputTokens":100,"outputTokens":0,"userEmail":"private@example.com"}]}`))
	require.NoError(t, err)
	assert.Contains(t, encoded, `"requests":2`)
	assert.NotContains(t, encoded, "private@example.com")
}

type typeSafeUsageTransport struct {
	status  int
	body    string
	request *http.Request
}

func (transport *typeSafeUsageTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.request = request
	return &http.Response{StatusCode: transport.status, Body: io.NopCloser(strings.NewReader(transport.body)), Header: http.Header{"Location": {"https://attacker.invalid/"}}, Request: request}, nil
}

func TestTypeSafeUsageRefreshDoesNotLeakUpstreamErrors(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, status := range []int{200, 302, 401, 403, 500} {
		transport := &typeSafeUsageTransport{status: status, body: `{"buckets":[],"debug":"secret-value"}`}
		http.DefaultTransport = transport
		encoded, err := refreshTypeSafeUsage(context.Background(), &model.TypeSafeUsageBinding{RequestCurl: typeSafeTestCurl})
		assert.Equal(t, "console.typesafe.ai", transport.request.URL.Host)
		assert.Equal(t, "session=test-only", transport.request.Header.Get("Cookie"))
		if status == 200 {
			require.NoError(t, err)
			assert.JSONEq(t, `[]`, encoded)
		} else {
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret-value")
		}
	}
}

func TestTypeSafeUsageBindingLifecyclePreservesCredentialsAndSnapshot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.TypeSafeUsageBinding{}))
	binding := &model.TypeSafeUsageBinding{Name: "test", RequestCurl: typeSafeTestCurl, Proxy: "http://private-proxy.invalid", Enabled: true}
	require.NoError(t, model.SaveTypeSafeUsageBinding(binding))
	require.NoError(t, model.UpdateTypeSafeUsageSnapshot(binding.Id, "[]", ""))
	require.NoError(t, model.UpdateTypeSafeUsageSnapshot(binding.Id, "", "登录已失效"))
	stored, err := model.GetTypeSafeUsageBinding(binding.Id)
	require.NoError(t, err)
	assert.Equal(t, "[]", stored.LastBuckets)
	assert.Positive(t, stored.LastRefreshedAt)
	assert.Equal(t, "登录已失效", stored.LastError)

	router := gin.New()
	router.PUT("/bindings/:id", SaveTypeSafeUsageBinding)
	request := httptest.NewRequest(http.MethodPut, "/bindings/"+strconv.Itoa(binding.Id), strings.NewReader(`{"name":"renamed","enabled":false}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success)
	assert.NotContains(t, response.Body.String(), "test-only")
	assert.NotContains(t, response.Body.String(), "private-proxy")
	stored, err = model.GetTypeSafeUsageBinding(binding.Id)
	require.NoError(t, err)
	assert.Equal(t, typeSafeTestCurl, stored.RequestCurl)
	assert.False(t, stored.Enabled)
	require.NoError(t, model.DeleteTypeSafeUsageBinding(binding.Id))
	_, err = model.GetTypeSafeUsageBinding(binding.Id)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestTypeSafeUsageAdminAccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.TypeSafeUsageBinding{}))
	previousDB, previousRedis, previousSecret := model.DB, common.RedisEnabled, common.SessionSecret
	model.DB, common.RedisEnabled, common.SessionSecret = db, false, "typesafe-access-test-secret"
	t.Cleanup(func() {
		model.DB, common.RedisEnabled, common.SessionSecret = previousDB, previousRedis, previousSecret
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	router := gin.New()
	router.GET("/bindings", middleware.AdminAuth(), GetTypeSafeUsageBindings)
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		user := &model.User{Username: fmt.Sprintf("typesafe-%d", role), AffCode: fmt.Sprintf("ts-%d", role), Password: "unused", Role: role, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
		require.NoError(t, db.Create(user).Error)
		bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "typesafe-test")
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodGet, "/bindings", nil)
		request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if role == common.RoleAdminUser {
			assert.Equal(t, http.StatusOK, response.Code)
		} else {
			assert.Equal(t, http.StatusForbidden, response.Code)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bindings", nil))
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
