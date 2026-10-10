package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestVolcengineChatNormalizesRequiredThinkingAfterOverrides(t *testing.T) {
	for _, test := range []struct {
		name           string
		model          string
		thinking       string
		effort         string
		override       string
		mapping        string
		channelType    int
		passThrough    bool
		globalPass     bool
		stream         bool
		wantModel      string
		wantThinking   string
		wantEffort     string
		wantDiagnostic bool
	}{
		{name: "Coding Plan 模型关闭思考降为低强度", model: "glm-5.3-flash", thinking: "disabled", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "流式请求关闭思考降为低强度", model: "glm-5.3-flash", thinking: "disabled", stream: true, wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "模型思考后缀关闭降为低强度", model: "glm-5.3-flash@thinking:off", wantModel: "glm-5.3-flash", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "显式 none 强度降为最低可用强度", model: "glm-5.3-flash", effort: "none", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "上游支持的 minimal 强度保持不变", model: "glm-5.3-flash", effort: "minimal", wantEffort: "minimal"},
		{name: "普通火山模型名称", model: "glm-5-3-flash", thinking: "disabled", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "带版本的模型名称", model: "glm-5-3-flash-260828", thinking: "disabled", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "最终参数覆盖不能重新禁用思考", model: "deepseek-v4-flash", override: `{"model":"glm-5.3-flash","thinking":{"type":"disabled"},"reasoning_effort":"none"}`, wantModel: "glm-5.3-flash", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "模型映射使用最终模型能力", model: "client-alias", thinking: "disabled", mapping: `{"client-alias":"glm-5.3-flash"}`, wantModel: "glm-5.3-flash", wantThinking: "enabled", wantEffort: "low", wantDiagnostic: true},
		{name: "显式开启与高强度保持不变", model: "glm-5.3-flash", thinking: "enabled", effort: "high", wantThinking: "enabled", wantEffort: "high"},
		{name: "未传思考参数保持上游默认", model: "glm-5.3-flash"},
		{name: "DeepSeek 仍可关闭思考", model: "deepseek-v4.1-flash", thinking: "disabled", wantThinking: "disabled"},
		{name: "GLM 5.2 仍可关闭思考", model: "glm-5-2-260617", thinking: "disabled", wantThinking: "disabled"},
		{name: "其他提供商保持请求语义", model: "glm-5.3-flash", channelType: constant.ChannelTypeOpenAI, thinking: "disabled", wantThinking: "disabled"},
		{name: "渠道透传保持请求原文", model: "glm-5.3-flash", thinking: "disabled", passThrough: true, wantThinking: "disabled"},
		{name: "全局透传保持请求原文", model: "glm-5.3-flash", thinking: "disabled", globalPass: true, wantThinking: "disabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := setupAdaptiveReasoning(t)
			require.NoError(t, i18n.Init())
			oldRetry := common.RetryTimes
			oldStreamingTimeout := constant.StreamingTimeout
			oldPass := model_setting.GetGlobalSettings().PassThroughRequestEnabled
			common.RetryTimes = 0
			constant.StreamingTimeout = 30
			model_setting.GetGlobalSettings().PassThroughRequestEnabled = test.globalPass
			t.Cleanup(func() {
				common.RetryTimes = oldRetry
				constant.StreamingTimeout = oldStreamingTimeout
				model_setting.GetGlobalSettings().PassThroughRequestEnabled = oldPass
			})
			ratio, err := common.Marshal(map[string]int{test.model: 1})
			require.NoError(t, err)
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(ratio)))
			ratio, err = common.Marshal(map[string]int{test.model: 0})
			require.NoError(t, err)
			require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(string(ratio)))
			wireBodies := make(chan []byte, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer main-test", r.Header.Get("Authorization"))
				upstreamBody, readErr := io.ReadAll(r.Body)
				if !assert.NoError(t, readErr) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				wireBodies <- upstreamBody
				w.Header().Set("Content-Type", "application/json")
				if test.wantDiagnostic && gjson.GetBytes(upstreamBody, "thinking.type").String() == "disabled" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":{"message":"thinking.type disabled is not supported","type":"InvalidParameter"}}`)
					return
				}
				if test.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"id\":\"stream-thinking\",\"object\":\"chat.completion.chunk\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"stream-thinking\",\"object\":\"chat.completion.chunk\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":0,\"total_tokens\":1000}}\n\ndata: [DONE]\n\n")
					return
				}
				_, _ = io.WriteString(w, `{"id":"thinking-response","object":"chat.completion","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":0,"total_tokens":1000}}`)
			}))
			t.Cleanup(upstream.Close)
			f.parent.Type = test.channelType
			if f.parent.Type == 0 {
				f.parent.Type = constant.ChannelTypeVolcEngine
			}
			f.parent.Models = test.model
			f.parent.BaseURL = &upstream.URL
			f.parent.ParamOverride = common.GetPointer(test.override)
			f.parent.ModelMapping = common.GetPointer(test.mapping)
			f.parent.SetSetting(dto.ChannelSettings{PassThroughBodyEnabled: test.passThrough})
			require.NoError(t, f.db.Save(f.parent).Error)
			router := gin.New()
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				defer common.CleanupBodyStorage(c)
				c.Set("id", 901)
				cache, cacheErr := model.GetUserCache(901)
				require.NoError(t, cacheErr)
				cache.WriteContext(c)
				common.SetContextKey(c, constant.ContextKeyTokenId, 902)
				common.SetContextKey(c, constant.ContextKeyTokenKey, "adaptive-test")
				common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
				common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
				require.Nil(t, middleware.SetupContextForSelectedChannel(c, f.parent, test.model))
				Relay(c, types.RelayFormatOpenAI)
			})
			body := map[string]any{
				"model": test.model, "messages": []map[string]string{{"role": "user", "content": "hello"}},
				"max_tokens": 256, "temperature": 0, "top_p": 0,
				"stream": test.stream,
			}
			if test.thinking != "" {
				body["thinking"] = map[string]string{"type": test.thinking}
			}
			if test.effort != "" {
				body["reasoning_effort"] = test.effort
			}
			payload, err := common.Marshal(body)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			upstreamBody := <-wireBodies
			if test.stream {
				assert.Contains(t, w.Body.String(), "[DONE]")
				assert.Contains(t, w.Body.String(), "OK")
			}
			wantModel := test.wantModel
			if wantModel == "" {
				wantModel = test.model
			}
			assert.Equal(t, wantModel, gjson.GetBytes(upstreamBody, "model").String())
			assert.Equal(t, test.wantThinking, gjson.GetBytes(upstreamBody, "thinking.type").String())
			assert.Equal(t, test.wantEffort, gjson.GetBytes(upstreamBody, "reasoning_effort").String())
			assert.Equal(t, "256", gjson.GetBytes(upstreamBody, "max_tokens").Raw)
			assert.Equal(t, "0", gjson.GetBytes(upstreamBody, "temperature").Raw)
			assert.Equal(t, "0", gjson.GetBytes(upstreamBody, "top_p").Raw)
			assert.Equal(t, "hello", gjson.GetBytes(upstreamBody, "messages.0.content").String())
			if test.passThrough || test.globalPass {
				assert.Equal(t, string(payload), string(upstreamBody))
			}
			var logs []model.Log
			require.NoError(t, f.db.Find(&logs).Error)
			require.Len(t, logs, 1)
			assert.Equal(t, test.model, logs[0].ModelName)
			assert.Equal(t, 1000, logs[0].Quota)
			if test.wantDiagnostic {
				assert.Contains(t, logs[0].Other, "thinking_disabled_unsupported")
			} else {
				assert.NotContains(t, logs[0].Other, "thinking_disabled_unsupported")
			}
		})
	}
}
