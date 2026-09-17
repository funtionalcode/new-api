package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTypeSafeIntegrationBeforeAfterAndBilling(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.SubscriptionPlan{}))
	oldDB, oldLogs := model.DB, model.LOG_DB
	oldRedis, oldBatch, oldCache, oldConsume := common.RedisEnabled, common.BatchUpdateEnabled, common.MemoryCacheEnabled, common.LogConsumeEnabled
	oldRatio, oldCompletion := ratio_setting.ModelRatio2JSONString(), ratio_setting.CompletionRatio2JSONString()
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.BatchUpdateEnabled, common.MemoryCacheEnabled, common.LogConsumeEnabled = false, false, false, true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogs
		common.RedisEnabled, common.BatchUpdateEnabled, common.MemoryCacheEnabled, common.LogConsumeEnabled = oldRedis, oldBatch, oldCache, oldConsume
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatio))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(oldCompletion))
		_ = sqlDB.Close()
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"jev-latest":0.021}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"jev-latest":0}`))
	service.InitHttpClient()
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "typesafe-user", Group: "default", Quota: 1000000, Status: 1}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 902, UserId: 901, Name: "typesafe-token", Key: "token-canary", Status: 1, UnlimitedQuota: true}).Error)
	states := make(chan map[string]any, 4)
	var failureMode atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/systemone", r.URL.Path)
		assert.Equal(t, "Bearer typesafe-secret", r.Header.Get("Authorization"))
		switch failureMode.Load() {
		case 1:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		case 2:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{},"usage":{"input_tokens":-1,"output_tokens":0}}`))
			return
		}
		var body map[string]any
		if assert.NoError(t, common.DecodeJson(r.Body, &body)) {
			states <- body["state"].(map[string]any)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{"check":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1000,"output_tokens":20}}`))
	}))
	defer upstream.Close()
	question := map[string]dto.TypeSafeQuestion{"check": {Type: "noul", Instructions: json.RawMessage(`"Is this helpful?"`)}}
	cfg := dto.TypeSafeIntegration{ChannelID: 904, Before: question, After: question}
	override, err := common.Marshal(map[string]any{"_typesafe": cfg})
	require.NoError(t, err)
	parentChannel := &model.Channel{Id: 903, Type: constant.ChannelTypeOpenAI, Name: "main", Models: "chat-model", Group: "default", Status: 1, Key: "main-secret", ParamOverride: common.GetPointer(string(override))}
	target := &model.Channel{Id: 904, Type: constant.ChannelTypeTypeSafe, Name: "eval", Models: "jev-latest", Group: "default", Status: 1, Key: "typesafe-secret", BaseURL: &upstream.URL, OpenUserIds: model.ChannelOpenUserIds{901}}
	require.NoError(t, db.Create(parentChannel).Error)
	require.NoError(t, db.Create(target).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", 901)
	cache, err := model.GetUserCache(901)
	require.NoError(t, err)
	cache.WriteContext(c)
	common.SetContextKey(c, constant.ContextKeyTokenId, 902)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "token-canary")
	common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, common.RequestIdKey, "parent-typesafe-test")
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, parentChannel, "chat-model"))
	request := &dto.GeneralOpenAIRequest{Model: "chat-model", Messages: []dto.Message{{Role: "user", Content: "hello"}}}
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
	require.NoError(t, err)
	prepareTypeSafeIntegration(c, info)
	require.Len(t, info.TypeSafeResults, 1)
	require.Equal(t, "success", info.TypeSafeResults[0]["status"], "%+v", info.TypeSafeResults)
	assert.Contains(t, (<-states)["request"], "hello")
	c.Header("Content-Type", "text/event-stream")
	stream := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\ndata: [DONE]\n\n")
	_, err = c.Writer.Write(stream)
	require.NoError(t, err)
	require.NotNil(t, info.TypeSafeAfter)
	info.TypeSafeAfter()
	require.Len(t, info.TypeSafeResults, 2)
	require.Equal(t, "success", info.TypeSafeResults[1]["status"], "%+v", info.TypeSafeResults)
	assert.Equal(t, "world", (<-states)["response"])
	assert.Equal(t, stream, recorder.Body.Bytes())
	var logs []model.Log
	require.NoError(t, db.Where("channel_id = ?", 904).Find(&logs).Error)
	require.Len(t, logs, 2)
	for _, log := range logs {
		assert.Equal(t, 21, log.Quota)
		assert.Equal(t, 1000, log.PromptTokens)
		assert.Contains(t, log.Other, "parent-typesafe-test")
	}
	var user model.User
	require.NoError(t, db.First(&user, 901).Error)
	assert.Equal(t, 1000000-42, user.Quota)
	// 上游失败或返回无效用量时退回预扣费用，保留主回答。
	failureConfig := cfg
	failureConfig.Model, failureConfig.TimeoutMS = "jev-latest", 5000
	for _, mode := range []int32{1, 2} {
		failureMode.Store(mode)
		failureInfo := *info
		failureInfo.TypeSafeResults = nil
		evaluateTypeSafeStage(c, &failureInfo, failureConfig, "after", question, map[string]any{"response": "world"}, false)
		require.Len(t, failureInfo.TypeSafeResults, 1)
		assert.Equal(t, "evaluation_failed", failureInfo.TypeSafeResults[0]["reason"])
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			var refundedUser model.User
			var refundedToken model.Token
			require.NoError(collect, db.First(&refundedUser, 901).Error)
			require.NoError(collect, db.First(&refundedToken, 902).Error)
			assert.Equal(collect, 1000000-42, refundedUser.Quota)
			assert.Equal(collect, 42, refundedToken.UsedQuota)
		}, time.Second, time.Millisecond)
		assert.Equal(t, stream, recorder.Body.Bytes())
	}
	// 撤销引用渠道权限后，不再发送评估，主回答仍保留。
	require.NoError(t, db.Model(target).Update("open_user_ids", model.ChannelOpenUserIds{999}).Error)
	evaluateTypeSafeStage(c, info, cfg, "before", question, map[string]any{"request": "hello"}, false)
	assert.Equal(t, "channel_unavailable_or_forbidden", info.TypeSafeResults[2]["reason"])
	assert.Empty(t, states)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"chat-model": true})
	evaluateTypeSafeStage(c, info, cfg, "before", question, map[string]any{"request": "hello"}, false)
	assert.Equal(t, "model_not_allowed", info.TypeSafeResults[3]["reason"])
	assert.Empty(t, states)
}
