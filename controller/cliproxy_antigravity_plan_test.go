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
	validQuota, validPlan := quotaResponse, planResponse
	for _, tt := range []struct {
		name, quota, plan, wantError string
		wantCalls                    int
	}{
		{"valid quota and plan", validQuota, validPlan, "", 2},
		{"quota forbidden", `{"status_code":403,"body":{"error":{"status":"PERMISSION_DENIED","message":"Verify your account to continue.","details":[{"reason":"VALIDATION_REQUIRED","metadata":{"validation_url":"https://accounts.google.com/verify?token=private-validation-token"}}]}}}`, validPlan, "403; PERMISSION_DENIED; VALIDATION_REQUIRED; Verify your account to continue.", 1},
		{"invalid quota", `{"status_code":200,"body":{"groups":[{"buckets":[{"bucketId":"gemini-5h"}]}]}}`, validPlan, "gemini-5h", 1},
		{"wrong provider format", `{"status_code":200,"body":{"plan_type":"pro"}}`, validPlan, "Antigravity", 1},
		{"plan forbidden", validQuota, `{"status_code":403,"body":{"error":{"message":"Plan access denied"}}}`, "403", 2},
		{"plan missing", validQuota, `{"status_code":200,"body":{}}`, "套餐", 2},
		{"missing management configuration", validQuota, validPlan, "登录密码", 0},
		{"recovery clears previous error", validQuota, validPlan, "", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quotaResponse, planResponse, requests = tt.quota, tt.plan, nil
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
			assert.Contains(t, response.Data.LastAntigravityQuota, "0.9804")
			assert.Equal(t, response.Data.LastPlanType, stored.LastPlanType)
			assert.Equal(t, response.Data.LastAntigravityQuota, stored.LastAntigravityQuota)
			assert.Greater(t, stored.LastRefreshedAt, int64(123))
		})
	}
}
