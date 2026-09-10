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
			assert.Equal(t, tt.want, fetchCliproxyAntigravityPlan(context.Background(), caller, "ag"))
		})
	}
}

func TestRefreshCliproxyAntigravityUsageStoresAndPreservesPlan(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CliproxyAuthFileBinding{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.Create(&model.CliproxyAuthFileBinding{Id: 1, UserId: 1, AuthIndex: "ag", AuthName: "account.json", LastPlanType: "antigravity", Enabled: true}).Error)
	planUnavailable := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request service.CliproxyAPICallRequest
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, "ag", request.AuthIndex)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "Bearer $TOKEN$", request.Header["Authorization"])
		if request.URL == "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist" {
			assert.JSONEq(t, `{"metadata":{"ideType":"ANTIGRAVITY"}}`, request.Data)
			if planUnavailable {
				_, _ = w.Write([]byte(`{"status_code":403,"body":"{}"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status_code":200,"body":{"paidTier":{"id":"g1-pro-tier","name":"Google AI Pro"},"currentTier":{"id":"free-tier","name":"Antigravity"}}}`))
			return
		}
		assert.Equal(t, "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", request.URL)
		_, _ = w.Write([]byte(`{"status_code":200,"body":{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.9804}]}]}}`))
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
	for _, unavailable := range []bool{false, true} {
		planUnavailable = unavailable
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bindings/1/refresh-usage", nil))
		var response struct {
			Success bool                          `json:"success"`
			Data    model.CliproxyAuthFileBinding `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		require.True(t, response.Success)
		assert.Equal(t, "Google AI Pro", response.Data.LastPlanType)
		assert.Empty(t, response.Data.LastError)
		assert.Contains(t, response.Data.LastAntigravityQuota, "0.9804")
		stored, err := model.GetCliproxyAuthFileBindingById(1)
		require.NoError(t, err)
		assert.Equal(t, "Google AI Pro", stored.LastPlanType)
		assert.Equal(t, "antigravity", stored.Provider)
	}
}
