package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFetchCliproxyAntigravityPlanPrefersPaidTier(t *testing.T) {
	for _, tt := range []struct{ name, body, want string }{
		{"paid tier", `{"paidTier":{"id":"g1-pro-tier","name":"Google AI Pro"},"currentTier":{"id":"free-tier","name":"Antigravity"}}`, "Google AI Pro"},
		{"free tier", `{"currentTier":{"id":"free-tier","name":"Antigravity"}}`, "Free"},
		{"unknown tier name", `{"paidTier":{"id":"future-tier","name":"Future Plan"}}`, "Future Plan"},
		{"tier ID only", `{"paidTier":{"id":"g1-pro-tier"}}`, "g1-pro-tier"},
		{"missing tier", `{}`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.UnmarshalJsonStr(tt.body, &body))
			caller := &fakeCliproxyAPICaller{responses: map[string]*service.CliproxyAPICallResponse{
				"https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist": {StatusCode: http.StatusOK, Body: body},
			}}
			plan, err := fetchCliproxyAntigravityPlan(context.Background(), caller, "ag")
			if tt.want == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, plan)
		})
	}
}

func TestRefreshCliproxyAntigravityUsageReportsErrorsWithoutFallback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CliproxyAuthFileBinding{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.Create(&model.CliproxyAuthFileBinding{Id: 1, UserId: 1, AuthIndex: "ag", AuthName: "antigravity-account.json", LastPlanType: "antigravity", Enabled: true}).Error)
	quotaResponse := `{"status_code":200,"body":{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.9804},{"bucketId":"gemini-weekly","remainingFraction":1}]}]}}`
	planResponse := `{"status_code":200,"body":{"paidTier":{"id":"g1-pro-tier","name":"Google AI Pro"},"currentTier":{"id":"free-tier","name":"Antigravity"}}}`
	modelsResponse := `{"status_code":200,"body":{"models":{"gemini-2.5-pro":{"quotaInfo":{"remainingFraction":1,"resetTime":"2026-10-23T10:43:03Z"}},"gemini-3-flash":{"quotaInfo":{"remainingFraction":1,"resetTime":"2026-10-23T10:40:00Z"}},"tab-flash":{"apiProvider":"API_PROVIDER_GOOGLE_GEMINI","quotaInfo":{"remainingFraction":0}},"chat-internal":{"apiProvider":"API_PROVIDER_INTERNAL","modelProvider":"MODEL_PROVIDER_GOOGLE","quotaInfo":{"remainingFraction":0}}},"tabModelIds":["tab-flash"]}}`
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request service.CliproxyAPICallRequest
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, "ag", request.AuthIndex)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
		requests = append(requests, request.URL)
		if request.URL == "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist" {
			assert.JSONEq(t, `{"metadata":{"ideType":"ANTIGRAVITY"}}`, request.Data)
			_, _ = w.Write([]byte(planResponse))
			return
		}
		if request.URL == "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels" {
			assert.JSONEq(t, `{"project":"aicode-consumers"}`, request.Data)
			_, _ = w.Write([]byte(modelsResponse))
			return
		}
		assert.Equal(t, "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", request.URL)
		_, _ = w.Write([]byte(quotaResponse))
	}))
	t.Cleanup(upstream.Close)
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{"CliproxyAPIBaseURL": upstream.URL, "CliproxyAPIPassword": "test-password"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})
	router := gin.New()
	router.POST("/bindings/:id/refresh-usage", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", common.RoleAdminUser)
		RefreshCliproxyAuthFileBindingUsage(c)
	})
	validQuota, validPlan, validModels := quotaResponse, planResponse, modelsResponse
	thirdPartyQuota := `{"status_code":200,"body":{"groups":[{"displayName":"Claude and GPT models","buckets":[{"bucketId":"3p-weekly","remainingFraction":1,"resetTime":"2026-10-30T02:44:43Z"},{"bucketId":"3p-5h","remainingFraction":0,"resetTime":"2026-10-23T08:25:14Z"}]}]}}`
	for _, tt := range []struct {
		name, quota, plan, models, wantError string
		wantCalls                            int
		wantBucketIDs                        []string
	}{
		{name: "valid quota and plan", quota: validQuota, plan: validPlan, wantCalls: 2, wantBucketIDs: []string{"gemini-5h", "gemini-weekly"}},
		{name: "Gemini model quota supplements third-party windows", quota: thirdPartyQuota, plan: validPlan, wantCalls: 3, wantBucketIDs: []string{"gemini-shared", "3p-5h", "3p-weekly"}},
		{name: "model quota forbidden", quota: thirdPartyQuota, models: `{"status_code":403,"body":{"error":{"message":"Gemini access denied"}}}`, wantError: "403", wantCalls: 2},
		{name: "model quota missing", quota: thirdPartyQuota, models: `{"status_code":200,"body":{"models":{}}}`, wantError: "Gemini", wantCalls: 2},
		{name: "quota forbidden", quota: `{"status_code":403,"body":{"error":{"status":"PERMISSION_DENIED","message":"Verify your account to continue.","details":[{"reason":"VALIDATION_REQUIRED","metadata":{"validation_url":"https://accounts.google.com/verify?token=private-validation-token"}}]}}}`, plan: validPlan, wantError: "403; PERMISSION_DENIED; VALIDATION_REQUIRED; Verify your account to continue.", wantCalls: 1},
		{name: "invalid quota", quota: `{"status_code":200,"body":{"groups":[{"buckets":[{"bucketId":"gemini-5h"}]}]}}`, plan: validPlan, wantError: "gemini-5h", wantCalls: 1},
		{name: "wrong provider format", quota: `{"status_code":200,"body":{"plan_type":"pro"}}`, plan: validPlan, wantError: "Antigravity", wantCalls: 1},
		{name: "plan forbidden", quota: validQuota, plan: `{"status_code":403,"body":{"error":{"message":"Plan access denied"}}}`, wantError: "403", wantCalls: 2},
		{name: "plan missing", quota: validQuota, plan: `{"status_code":200,"body":{}}`, wantError: "套餐", wantCalls: 2},
		{name: "missing management configuration", quota: validQuota, plan: validPlan, wantError: "登录密码", wantCalls: 0},
		{name: "recovery clears previous error", quota: validQuota, plan: validPlan, wantCalls: 2, wantBucketIDs: []string{"gemini-5h", "gemini-weekly"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quotaResponse, planResponse, requests = tt.quota, tt.plan, nil
			modelsResponse = firstNonEmpty(tt.models, validModels)
			common.OptionMapRWMutex.Lock()
			common.OptionMap["CliproxyAPIPassword"] = "test-password"
			if tt.wantCalls == 0 {
				common.OptionMap["CliproxyAPIPassword"] = ""
			}
			common.OptionMapRWMutex.Unlock()
			require.NoError(t, db.Model(&model.CliproxyAuthFileBinding{}).Where("id = ?", 1).Updates(map[string]any{
				"last_plan_type": "Google AI Ultra", "last_antigravity_quota": "old snapshot", "last_refreshed_at": 123, "last_error": "previous error",
			}).Error)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bindings/1/refresh-usage", nil))
			var response struct {
				Success bool                          `json:"success"`
				Message string                        `json:"message"`
				Data    model.CliproxyAuthFileBinding `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Len(t, requests, tt.wantCalls)
			stored, err := model.GetCliproxyAuthFileBindingById(1)
			require.NoError(t, err)
			assert.Equal(t, "antigravity", stored.Provider)
			assert.NotContains(t, recorder.Body.String(), "private-validation-token")
			if tt.wantError != "" {
				require.False(t, response.Success)
				assert.Contains(t, response.Message, tt.wantError)
				assert.Equal(t, response.Message, stored.LastError)
				assert.NotContains(t, stored.LastError, "private-validation-token")
				assert.Empty(t, stored.LastPlanType)
				assert.Empty(t, stored.LastAntigravityQuota)
				assert.Equal(t, int64(123), stored.LastRefreshedAt)
				return
			}
			require.True(t, response.Success)
			assert.Equal(t, "Google AI Pro", response.Data.LastPlanType)
			assert.Empty(t, response.Data.LastError)
			var buckets []cliproxyAntigravityQuotaBucket
			require.NoError(t, common.UnmarshalJsonStr(response.Data.LastAntigravityQuota, &buckets))
			require.Len(t, buckets, len(tt.wantBucketIDs))
			for i, id := range tt.wantBucketIDs {
				assert.Equal(t, id, buckets[i].BucketID)
				require.NotNil(t, buckets[i].RemainingFraction)
			}
			if tt.wantBucketIDs[0] == "gemini-shared" {
				assert.Equal(t, 1.0, *buckets[0].RemainingFraction)
				assert.Equal(t, int64(1792752183), buckets[0].ResetAt)
				assert.Zero(t, *buckets[1].RemainingFraction)
				assert.Equal(t, 1.0, *buckets[2].RemainingFraction)
				assert.Equal(t, int64(1792743914), buckets[1].ResetAt)
			} else {
				assert.Equal(t, 0.9804, *buckets[0].RemainingFraction)
			}
			assert.Equal(t, response.Data.LastPlanType, stored.LastPlanType)
			assert.Equal(t, response.Data.LastAntigravityQuota, stored.LastAntigravityQuota)
			assert.Greater(t, stored.LastRefreshedAt, int64(123))
		})
	}
}
