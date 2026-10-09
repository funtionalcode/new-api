package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	rootdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCrossChannelModelMapping(t *testing.T) *adaptiveReasoningFixture {
	t.Helper()
	originalDB := model.DB
	initModelListColumnNames(t)
	model.DB = originalDB
	f := setupAdaptiveReasoning(t)
	require.NoError(t, f.db.AutoMigrate(&model.Ability{}))
	f.parent.Models = "deepseek-v4-flash"
	f.parent.ModelMapping = common.GetPointer(`{"deepseek-v4-flash":"qwen3.8-27b"}`)
	f.parent.SetSetting(dto.ChannelSettings{ModelMappingChannels: map[string]int{"deepseek-v4-flash": f.target.Id}})
	f.target.Type = constant.ChannelTypeOpenAI
	f.target.Models = "qwen3.8-27b"
	f.target.ModelMapping = common.GetPointer(`{"qwen3.8-27b":"qwen-upstream"}`)
	f.target.Key = "target-only-secret"
	f.target.SetSetting(dto.ChannelSettings{})
	require.NoError(t, f.db.Save(f.parent).Error)
	require.NoError(t, f.db.Save(f.target).Error)
	for _, channel := range []*model.Channel{f.parent, f.target} {
		require.NoError(t, f.db.Create(&model.Ability{Group: "default", Model: channel.Models, ChannelId: channel.Id, Enabled: true}).Error)
	}
	return f
}

func TestCrossChannelModelMappingUsesTargetCredentialsAndOriginalBilling(t *testing.T) {
	f := setupCrossChannelModelMapping(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"deepseek-v4-flash":1,"qwen3.8-27b":9}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"deepseek-v4-flash":0,"qwen3.8-27b":0}`))
	oldRetry := common.RetryTimes
	common.RetryTimes = 0
	t.Cleanup(func() { common.RetryTimes = oldRetry })
	var sourceCalls atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(source.Close)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer target-only-secret", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		if !assert.NoError(t, common.DecodeJson(r.Body, &request)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, "qwen-upstream", request.Model)
		assert.Equal(t, "hello", request.Messages[0].StringContent())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"mapped-response","object":"chat.completion","model":"qwen-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":0,"total_tokens":1000}}`)
	}))
	t.Cleanup(target.Close)
	f.parent.BaseURL, f.target.BaseURL = &source.URL, &target.URL
	require.NoError(t, f.db.Save(f.parent).Error)
	require.NoError(t, f.db.Save(f.target).Error)
	router := gin.New()
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Set("id", 901)
		cache, err := model.GetUserCache(901)
		require.NoError(t, err)
		cache.WriteContext(c)
		common.SetContextKey(c, constant.ContextKeyTokenId, 902)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "adaptive-test")
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	}, middleware.Distribute(), func(c *gin.Context) { Relay(c, types.RelayFormatOpenAI) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "OK")
	assert.Equal(t, int32(0), sourceCalls.Load())
	var logs []model.Log
	require.NoError(t, f.db.Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, f.target.Id, logs[0].ChannelId)
	assert.Equal(t, "deepseek-v4-flash", logs[0].ModelName)
	assert.Equal(t, 1000, logs[0].Quota)
	assert.NotContains(t, logs[0].Other, "target-only-secret")
}

func TestCrossChannelModelMappingValidatesExposedModelsBeforeSaving(t *testing.T) {
	f := setupCrossChannelModelMapping(t)
	require.NoError(t, validateChannel(f.parent, false))
	for _, test := range []struct {
		name     string
		source   string
		targetID int
	}{
		{"当前渠道", "deepseek-v4-flash", f.parent.Id},
		{"不存在的渠道", "deepseek-v4-flash", 999999},
		{"未开放的源模型", "missing-model", f.target.Id},
	} {
		t.Run(test.name, func(t *testing.T) {
			f.parent.SetSetting(dto.ChannelSettings{ModelMappingChannels: map[string]int{test.source: test.targetID}})
			assert.Error(t, validateChannel(f.parent, false))
		})
	}
	f.parent.SetSetting(dto.ChannelSettings{ModelMappingChannels: map[string]int{"deepseek-v4-flash": f.target.Id}})
	f.parent.ModelMapping = common.GetPointer(`{"deepseek-v4-flash":"missing-target"}`)
	assert.Error(t, validateChannel(f.parent, false))
}

func TestCrossChannelModelMappingRejectsUnavailableOrForbiddenTargets(t *testing.T) {
	for _, test := range []struct {
		name   string
		modify func(*adaptiveReasoningFixture, *gin.Context)
		status int
	}{
		{"目标渠道禁用", func(f *adaptiveReasoningFixture, c *gin.Context) { f.target.Status = 2 }, http.StatusServiceUnavailable},
		{"目标渠道未开放给用户", func(f *adaptiveReasoningFixture, c *gin.Context) {
			f.target.OpenUserIds = model.ChannelOpenUserIds{999}
		}, http.StatusForbidden},
		{"目标模型不在当前分组", func(f *adaptiveReasoningFixture, c *gin.Context) {
			require.NoError(t, f.db.Where("channel_id = ?", f.target.Id).Delete(&model.Ability{}).Error)
		}, http.StatusForbidden},
		{"映射循环", func(f *adaptiveReasoningFixture, c *gin.Context) {
			f.target.ModelMapping = common.GetPointer(`{"qwen3.8-27b":"deepseek-v4-flash"}`)
			f.target.SetSetting(dto.ChannelSettings{ModelMappingChannels: map[string]int{"qwen3.8-27b": f.parent.Id}})
		}, http.StatusBadRequest},
		{"令牌指定渠道", func(f *adaptiveReasoningFixture, c *gin.Context) {
			service.GetChannelConstraints(c).AddPin(rootdto.ChannelPin{ChannelId: f.parent.Id, Source: rootdto.PinSourceToken})
		}, http.StatusForbidden},
		{"目标渠道协议不兼容", func(f *adaptiveReasoningFixture, c *gin.Context) { f.target.Type = constant.ChannelTypeTypeSafe }, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := setupCrossChannelModelMapping(t)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Set("id", 901)
			test.modify(f, c)
			require.NoError(t, f.db.Save(f.target).Error)
			_, apiErr := service.ResolveChannelModelMapping(c, f.parent, "deepseek-v4-flash", "default")
			require.NotNil(t, apiErr)
			assert.Equal(t, test.status, apiErr.StatusCode)
		})
	}
}

func TestCrossChannelModelMappingResetsTargetOnRetryAndKeepsCompactBillingName(t *testing.T) {
	f := setupCrossChannelModelMapping(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	c.Set("id", 901)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	channel, _, selectErr := service.SelectChannelForRequest(c, "deepseek-v4-flash", &service.RetryParam{Ctx: c, TokenGroup: "default", ModelName: "deepseek-v4-flash", RequestPath: c.Request.URL.Path})
	require.Nil(t, selectErr)
	require.Equal(t, f.target.Id, channel.Id)
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, channel, "deepseek-v4-flash"))
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-v4-flash", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-v4-flash"}}
	require.NoError(t, helper.ModelMappedHelper(c, info, nil))
	assert.Equal(t, "qwen-upstream", info.UpstreamModelName)
	assert.Equal(t, "deepseek-v4-flash", info.OriginModelName)
	compactInfo := &relaycommon.RelayInfo{OriginModelName: "deepseek-v4-flash.compact", RelayMode: relayconstant.RelayModeResponsesCompact, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-v4-flash.compact"}}
	require.NoError(t, helper.ModelMappedHelper(c, compactInfo, nil))
	assert.Equal(t, "qwen-upstream", compactInfo.UpstreamModelName)
	assert.Equal(t, "deepseek-v4-flash.compact", compactInfo.OriginModelName)
	f.parent.SetSetting(dto.ChannelSettings{})
	require.NoError(t, f.db.Save(f.parent).Error)
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "default", ModelName: "deepseek-v4-flash", RequestPath: c.Request.URL.Path})
	require.NoError(t, err)
	assert.Equal(t, f.parent.Id, channel.Id)
	assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyModelMappingTarget))
}

func TestCrossChannelModelMappingWebsocketSelectionWithAndWithoutCache(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "数据库选择", true: "缓存选择"}[cached], func(t *testing.T) {
			f := setupCrossChannelModelMapping(t)
			common.MemoryCacheEnabled = cached
			if cached {
				model.InitChannelCache()
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			c.Set("id", 901)
			cache, err := model.GetUserCache(901)
			require.NoError(t, err)
			cache.WriteContext(c)
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			channel, apiErr := middleware.SelectChannelForWebsocketRequest(c, "deepseek-v4-flash")
			require.Nil(t, apiErr)
			assert.Equal(t, f.target.Id, channel.Id)
			assert.Equal(t, "target-only-secret", common.GetContextKeyString(c, constant.ContextKeyChannelKey))
			assert.Equal(t, "qwen3.8-27b", common.GetContextKeyString(c, constant.ContextKeyModelMappingTarget))
			assert.Equal(t, "deepseek-v4-flash", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		})
	}
}
