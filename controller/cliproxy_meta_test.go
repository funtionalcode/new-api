package controller

import (
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

func TestRefreshCliproxyMetaQuota(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CliproxyAuthFileBinding{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.Create(&model.CliproxyAuthFileBinding{
		Id: 1, UserId: 1, AuthIndex: "meta-index", Provider: "meta", AuthName: "stale-name.json", Enabled: true,
	}).Error)

	const validCredential = `{"type":"meta","dca_token":"dca:test-secret","access_token":"wrong-model-key"}`
	const validQuota = `{"subs_tier_name":"Pro Tier","is_subs_active":false,"subs_usage":{"tier":"other tier","window":{"used_percent":0,"resets_at":1793000000,"window_duration_mins":120},"weekly":{"used_percent":"15.125","resets_at":"1793500000"}},"api_key":"private-minted-key","user_email":"private@example.com"}`
	var credential, quota, remoteProvider, remoteName string
	var status, managementStatus, downloadStatus, userID, role int
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		assert.Equal(t, "Bearer management-password", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/management/auth-files":
			assert.Equal(t, http.MethodGet, r.Method)
			raw, marshalErr := common.Marshal(map[string]any{"files": []map[string]any{
				{"auth_index": "other-index", "name": "other.json", "provider": "meta"},
				{"auth_index": "meta-index", "name": remoteName, "provider": remoteProvider},
			}})
			assert.NoError(t, marshalErr)
			_, _ = w.Write(raw)
		case "/v0/management/auth-files/download":
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "current-account.json", r.URL.Query().Get("name"))
			w.WriteHeader(downloadStatus)
			_, _ = w.Write([]byte(credential))
		case "/v0/management/api-call":
			assert.Equal(t, http.MethodPost, r.Method)
			var payload service.CliproxyAPICallRequest
			if !assert.NoError(t, common.DecodeJson(r.Body, &payload)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assert.Equal(t, "meta-index", payload.AuthIndex)
			assert.Equal(t, "https://api.meta.ai/muse-code/key", payload.URL)
			assert.Equal(t, http.MethodPost, payload.Method)
			assert.Equal(t, map[string]string{
				"Authorization": "Bearer dca:test-secret", "Content-Type": "application/json", "Accept": "application/json",
				"x-api-version": "1.0.0", "User-Agent": "muse-code/1.0.2",
			}, payload.Header)
			assert.JSONEq(t, `{}`, payload.Data)
			w.WriteHeader(managementStatus)
			body, marshalErr := common.Marshal(map[string]any{"status_code": status, "body": quota})
			assert.NoError(t, marshalErr)
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected CPA request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)
	common.OptionMapRWMutex.Lock()
	originalOptions := common.OptionMap
	common.OptionMap = map[string]string{"CliproxyAPIBaseURL": upstream.URL, "CliproxyAPIPassword": "management-password"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptions
		common.OptionMapRWMutex.Unlock()
	})
	router := gin.New()
	router.POST("/bindings/:id/refresh-usage", func(c *gin.Context) {
		c.Set("id", userID)
		c.Set("role", role)
		RefreshCliproxyAuthFileBindingUsage(c)
	})
	for _, tt := range []struct {
		name, credential, quota, provider, filename, wantError string
		status, managementStatus, downloadStatus, userID, role int
		wantCalls                                              int
		unknown, fallbackPlan, clamped, noConfig               bool
	}{
		{name: "current DCA contract and safe snapshot", wantCalls: 3},
		{name: "unknown quotas clear old plan without inventing percentages", quota: `{}`, unknown: true, wantCalls: 3},
		{name: "plan fallback and bounded percentages", quota: `{"subs_usage":{"tier":"Free Tier","window":{"used_percent":-5},"weekly":{"used_percent":105}}}`, fallbackPlan: true, clamped: true, wantCalls: 3},
		{name: "invalid numbers remain unknown", quota: `{"subs_usage":{"window":{"used_percent":"NaN","window_duration_mins":0,"resets_at":1793000000000},"weekly":{"used_percent":null}}}`, unknown: true, wantCalls: 3},
		{name: "age verification required", status: 403, quota: `{"title":"Age Requirement Not Met","detail":"private-minted-key","code":4705001,"status":403,"api_key":"private-minted-key"}`, wantError: "Meta requires a verified 18+ account. Verify your age or add a payment method, then retry.", wantCalls: 3},
		{name: "expired authorization", status: 401, quota: `{"detail":"dca:test-secret"}`, wantError: "Meta authorization expired or was denied. Sign in to Meta again.", wantCalls: 3},
		{name: "forbidden without error details", status: 403, quota: `{}`, wantError: "Meta authorization expired or was denied. Sign in to Meta again.", wantCalls: 3},
		{name: "management denial is not Meta account denial", managementStatus: 403, wantError: "Meta quota request failed. Try again later.", wantCalls: 3},
		{name: "no model key fallback", credential: `{"type":"meta","access_token":"wrong-model-key","api_key":"private-minted-key"}`, wantError: "Meta credential is missing a valid DCA token. Sign in to Meta again.", wantCalls: 2},
		{name: "invalid DCA rejected before forwarding", credential: `{"type":"meta","dca_token":"dca:test\nsecret"}`, wantError: "Meta credential is missing a valid DCA token. Sign in to Meta again.", wantCalls: 2},
		{name: "downloaded provider mismatch", credential: `{"type":"codex","dca_token":"dca:test-secret"}`, wantError: "Failed to load the Meta credential. Refresh the credential list and try again.", wantCalls: 2},
		{name: "current selected provider mismatch", provider: "codex", wantError: "Failed to load the Meta credential. Refresh the credential list and try again.", wantCalls: 1},
		{name: "download cannot select a path", filename: "../other.json", wantError: "Failed to load the Meta credential. Refresh the credential list and try again.", wantCalls: 1},
		{name: "download denied", downloadStatus: 403, wantError: "Failed to download the Meta credential.", wantCalls: 2},
		{name: "invalid quota response", quota: `{"error":{"message":"private-minted-key"}}`, wantError: "Invalid Meta quota response.", wantCalls: 3},
		{name: "owner check before secret access", userID: 2, wantError: "无权刷新", wantCalls: 0},
		{name: "admin may refresh another owner", userID: 2, role: common.RoleAdminUser, wantCalls: 3},
		{name: "invalid management configuration clears stale data", noConfig: true, wantError: "Meta quota request failed. Try again later.", wantCalls: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			credential, quota = firstNonEmpty(tt.credential, validCredential), firstNonEmpty(tt.quota, validQuota)
			remoteProvider, remoteName = firstNonEmpty(tt.provider, "meta"), firstNonEmpty(tt.filename, "current-account.json")
			status, managementStatus, downloadStatus = tt.status, tt.managementStatus, tt.downloadStatus
			if status == 0 {
				status = http.StatusOK
			}
			if managementStatus == 0 {
				managementStatus = http.StatusOK
			}
			if downloadStatus == 0 {
				downloadStatus = http.StatusOK
			}
			userID, role = tt.userID, tt.role
			if userID == 0 {
				userID = 1
			}
			requests = nil
			common.OptionMapRWMutex.Lock()
			common.OptionMap["CliproxyAPIPassword"] = "management-password"
			if tt.noConfig {
				common.OptionMap["CliproxyAPIPassword"] = ""
			}
			common.OptionMapRWMutex.Unlock()
			require.NoError(t, db.Model(&model.CliproxyAuthFileBinding{}).Where("id = ?", 1).Updates(map[string]any{
				"last_meta_quota": "old snapshot", "last_plan_type": "old plan", "last_refreshed_at": 123, "last_error": "previous error",
			}).Error)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/bindings/1/refresh-usage", nil))
			var response struct {
				Success    bool                          `json:"success"`
				Message    string                        `json:"message"`
				MessageKey string                        `json:"message_key"`
				Data       model.CliproxyAuthFileBinding `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Len(t, requests, tt.wantCalls)
			stored, err := model.GetCliproxyAuthFileBindingById(1)
			require.NoError(t, err)
			for _, secret := range []string{"dca:test-secret", "wrong-model-key", "private-minted-key", "private@example.com"} {
				assert.NotContains(t, recorder.Body.String(), secret)
				assert.NotContains(t, stored.LastMetaQuota+stored.LastError, secret)
			}
			if tt.wantError != "" {
				require.False(t, response.Success)
				assert.Contains(t, response.Message, tt.wantError)
				assert.Equal(t, int64(123), stored.LastRefreshedAt)
				if tt.userID == 2 && tt.role == 0 {
					assert.Equal(t, "old snapshot", stored.LastMetaQuota)
					assert.Equal(t, "previous error", stored.LastError)
					return
				}
				assert.Equal(t, response.Message, response.MessageKey)
				assert.Equal(t, response.Message, stored.LastError)
				assert.Empty(t, stored.LastPlanType)
				assert.Empty(t, stored.LastMetaQuota)
				return
			}
			require.True(t, response.Success)
			assert.Empty(t, stored.LastError)
			assert.Greater(t, stored.LastRefreshedAt, int64(123))
			assert.Equal(t, stored.LastMetaQuota, response.Data.LastMetaQuota)
			var snapshot service.CliproxyMetaQuota
			require.NoError(t, common.UnmarshalJsonStr(stored.LastMetaQuota, &snapshot))
			if tt.unknown {
				assert.Nil(t, snapshot.Window.UsedPercent)
				assert.Nil(t, snapshot.Weekly.UsedPercent)
				assert.Nil(t, snapshot.Window.DurationMinutes)
				assert.Nil(t, snapshot.Window.ResetAt)
				assert.Empty(t, stored.LastPlanType)
				return
			}
			require.NotNil(t, snapshot.Window.UsedPercent)
			require.NotNil(t, snapshot.Weekly.UsedPercent)
			assert.Equal(t, 0.0, *snapshot.Window.UsedPercent)
			if tt.clamped {
				assert.Equal(t, 100.0, *snapshot.Weekly.UsedPercent)
			} else {
				assert.Equal(t, 15.125, *snapshot.Weekly.UsedPercent)
				require.NotNil(t, snapshot.Window.DurationMinutes)
				assert.Equal(t, 120.0, *snapshot.Window.DurationMinutes)
				require.NotNil(t, snapshot.Window.ResetAt)
				assert.Equal(t, int64(1793000000), *snapshot.Window.ResetAt)
				require.NotNil(t, snapshot.SubscriptionActive)
				assert.False(t, *snapshot.SubscriptionActive)
			}
			if tt.fallbackPlan {
				assert.Equal(t, "Free Tier", stored.LastPlanType)
			} else {
				assert.Equal(t, "Pro Tier", stored.LastPlanType)
			}
			update := model.CliproxyAuthFileBindingUpdate{UserId: 1, Username: "owner", AuthIndex: "meta-index", AuthName: "current-account.json", Provider: "meta", Enabled: true}
			require.NoError(t, model.ValidateCliproxyAuthFileBindingUpdate(update))
			edited, err := model.UpdateCliproxyAuthFileBinding(1, update)
			require.NoError(t, err)
			assert.Equal(t, stored.LastMetaQuota, edited.LastMetaQuota)
		})
	}
}
