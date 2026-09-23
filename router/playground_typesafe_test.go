package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundTypeSafeUsesAuthenticatedUserAndNativeProtocol(t *testing.T) {
	require.NoError(t, i18n.Init())
	setupRelayRouterTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Log{}, &model.UserSession{}, &model.UserSubscription{}))
	oldSecret, oldCache, oldBatch, oldConsume := common.SessionSecret, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	oldRatio, oldCompletion := ratio_setting.ModelRatio2JSONString(), ratio_setting.CompletionRatio2JSONString()
	oldGroups, oldUsableGroups := ratio_setting.GroupRatio2JSONString(), setting.UserUsableGroups2JSONString()
	common.SessionSecret, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = "playground-typesafe-test", false, false, true
	t.Cleanup(func() {
		common.SessionSecret, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldSecret, oldCache, oldBatch, oldConsume
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatio))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(oldCompletion))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsableGroups))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"jev-latest":0.021}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"jev-latest":0}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	service.InitHttpClient()
	user := model.User{Username: "jev-playground-user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", Quota: 1000000, AuthVersion: 1}
	require.NoError(t, model.DB.Create(&user).Error)
	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	var calls atomic.Int32
	upstreamBody := `{"model":"jev-latest","answers":{"sentiment":{"type":"choice","choice":"positive"},"satisfied":{"type":"noul","noul":0.9},"rating":{"type":"score","score":2}},"usage":{"input_tokens":1000,"output_tokens":20}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/v1/systemone", r.URL.Path)
		assert.Equal(t, "Bearer upstream-test-key", r.Header.Get("Authorization"))
		var payload map[string]any
		if assert.NoError(t, common.DecodeJson(r.Body, &payload)) {
			assert.Equal(t, "jev-latest", payload["model"])
			assert.Equal(t, map[string]any{"review": "good"}, payload["state"])
			assert.NotContains(t, payload, "messages")
			assert.NotContains(t, payload, "group")
			assert.Len(t, payload["questions"], 3)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamBody))
	}))
	t.Cleanup(upstream.Close)
	channel := model.Channel{Type: constant.ChannelTypeTypeSafe, Status: common.ChannelStatusEnabled, Name: "Jev", Models: "jev-latest", Group: "vip", Key: "upstream-test-key", BaseURL: &upstream.URL}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "vip", Model: "jev-latest", ChannelId: channel.Id, Enabled: true}).Error)
	engine := gin.New()
	SetRelayRouter(engine)
	body := `{"model":"jev-latest","group":"vip","state":{"review":"good"},"questions":{"sentiment":{"type":"choice","instructions":"Classify","criteria":{"positive":null,"negative":null}},"satisfied":{"type":"noul","instructions":"Satisfied?"},"rating":{"type":"score","instructions":"Rate","criteria":["low","medium","high"]}}}`
	for _, tc := range []struct {
		name   string
		token  string
		body   string
		status int
	}{
		{"未登录", "", body, http.StatusUnauthorized},
		{"无效会话", "invalid-session", body, http.StatusUnauthorized},
		{"不允许的分组", bundle.AccessToken, strings.Replace(body, `"group":"vip"`, `"group":"forbidden"`, 1), http.StatusForbidden},
		{"问题缺少选项", bundle.AccessToken, strings.Replace(body, `"criteria":{"positive":null,"negative":null}`, `"criteria":{}`, 1), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/pg/systemone", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			assert.Zero(t, calls.Load())
		})
	}
	request := httptest.NewRequest(http.MethodPost, "/pg/systemone", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.JSONEq(t, upstreamBody, recorder.Body.String())
	assert.EqualValues(t, 1, calls.Load())
	var chargedUser model.User
	require.NoError(t, model.DB.First(&chargedUser, user.Id).Error)
	assert.Equal(t, 1000000-42, chargedUser.Quota)
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", user.Id, 2).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, 42, logs[0].Quota)
	assert.Equal(t, "vip", logs[0].Group)
	assert.Equal(t, "playground-vip", logs[0].TokenName)
	assert.NotContains(t, logs[0].Other, "upstream-test-key")

	// 已撤销的登录会话不能借游乐场入口继续调用或扣费。
	revoked, err := model.RevokeUserSession(user.Id, bundle.Session.SID, "test")
	require.NoError(t, err)
	require.True(t, revoked)
	revokedRequest := httptest.NewRequest(http.MethodPost, "/pg/systemone", strings.NewReader(body))
	revokedRequest.Header.Set("Content-Type", "application/json")
	revokedRequest.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	revokedResponse := httptest.NewRecorder()
	engine.ServeHTTP(revokedResponse, revokedRequest)
	assert.Equal(t, http.StatusUnauthorized, revokedResponse.Code)
	assert.EqualValues(t, 1, calls.Load())
}
