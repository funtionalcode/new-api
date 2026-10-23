package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXAISpeechRelayReturnsAudioAndSettlesCharacters(t *testing.T) {
	for _, scenario := range []struct {
		name, model, path, format      string
		countToken                     bool
		upstreamStatus, expectedStatus int
		emptyAudio                     bool
		jsonAudio                      bool
	}{
		{name: "游乐场默认模型", model: "grok-tts", path: "/pg/audio/speech", countToken: true, upstreamStatus: 200, expectedStatus: 200},
		{name: "游乐场版本名称关闭预估", model: "grok-voice-tts-1.0", path: "/pg/audio/speech", upstreamStatus: 200, expectedStatus: 200},
		{name: "兼容接口", model: "grok-voice-tts-1.0", path: "/v1/audio/speech", countToken: true, upstreamStatus: 200, expectedStatus: 200},
		{name: "非法输出格式", model: "grok-tts", path: "/pg/audio/speech", format: "opus", countToken: true, expectedStatus: 400},
		{name: "上游拒绝退还预留", model: "grok-tts", path: "/pg/audio/speech", countToken: true, upstreamStatus: 429, expectedStatus: 429},
		{name: "上游空音频退还预留", model: "grok-tts", path: "/pg/audio/speech", countToken: true, upstreamStatus: 200, expectedStatus: 500, emptyAudio: true},
		{name: "上游返回JSON退还预留", model: "grok-tts", path: "/pg/audio/speech", countToken: true, upstreamStatus: 200, expectedStatus: 500, jsonAudio: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			user, token := setupResponsesWSRequestTest(t)
			previousBatch, previousLogs, previousCount, previousQuota, previousRetry := common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit, common.RetryTimes
			common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit, common.RetryTimes = false, true, scenario.countToken, 500000, 0
			saved := map[string]string{}
			require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
				switch key {
				case "billing_setting.billing_mode", "billing_setting.billing_expr", "group_ratio_setting.group_ratio":
					saved[key] = value
				}
				return nil
			}))
			t.Cleanup(func() {
				common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit, common.RetryTimes = previousBatch, previousLogs, previousCount, previousQuota, previousRetry
				require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
			})
			modes, err := common.Marshal(map[string]string{scenario.model: "tiered_expr"})
			require.NoError(t, err)
			expressions, err := common.Marshal(map[string]string{scenario.model: `tier("tts", p * 15 + c * 999)`})
			require.NoError(t, err)
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting.billing_mode": string(modes), "billing_setting.billing_expr": string(expressions), "group_ratio_setting.group_ratio": `{"default":1}`}))
			require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
			require.NoError(t, model.LOG_DB.AutoMigrate(&model.Log{}))
			require.NoError(t, model.DB.Model(user).Updates(map[string]any{"quota": 100000, "setting": `{"billing_preference":"wallet_only"}`}).Error)
			require.NoError(t, model.DB.Model(token).Update("remain_quota", 1000).Error)
			var calls atomic.Int32
			audio := []byte{'I', 'D', '3', 0, 0xff, 1, 2}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, "/v1/tts", r.URL.Path)
				var request map[string]any
				assert.NoError(t, common.DecodeJson(r.Body, &request))
				assert.Equal(t, " 你好，Grok 😀! ", request["text"])
				assert.Equal(t, "ara", request["voice_id"])
				assert.Equal(t, "auto", request["language"])
				assert.NotContains(t, request, "model")
				if scenario.upstreamStatus != http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(scenario.upstreamStatus)
					_, _ = w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
					return
				}
				w.Header().Set("Content-Type", "audio/mpeg")
				if scenario.jsonAudio {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"error":"no audio"}`))
					return
				}
				if !scenario.emptyAudio {
					_, _ = w.Write(audio)
				}
			}))
			defer upstream.Close()
			channel := &model.Channel{Name: "xai-tts", Key: "upstream-secret", Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeXai, Group: "default", Models: scenario.model, BaseURL: &upstream.URL}
			require.NoError(t, model.DB.Create(channel).Error)
			require.NoError(t, model.DB.Create(&model.Ability{ChannelId: channel.Id, Model: scenario.model, Group: "default", Enabled: true}).Error)
			engine := gin.New()
			engine.POST(scenario.path, middleware.BodyStorageCleanup(), middleware.TokenAuth(), middleware.Distribute(), func(c *gin.Context) {
				Relay(c, types.RelayFormatOpenAIAudio)
			})
			body := fmt.Sprintf(`{"model":%q,"input":" 你好，Grok 😀! ","voice":"alloy","response_format":%q}`, scenario.model, scenario.format)
			request := httptest.NewRequest(http.MethodPost, scenario.path, strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer sk-"+token.Key)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, scenario.expectedStatus, response.Code, "%s", response.Body.String())
			var logs []model.Log
			require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).Find(&logs).Error)
			quota := 0
			if scenario.expectedStatus == http.StatusOK {
				assert.Equal(t, audio, response.Body.Bytes())
				assert.Equal(t, "audio/mpeg", response.Header().Get("Content-Type"))
				require.Len(t, logs, 1)
				assert.Equal(t, 12, logs[0].PromptTokens)
				assert.Zero(t, logs[0].CompletionTokens)
				quota = 90
				assert.Equal(t, quota, logs[0].Quota)
			} else {
				assert.Empty(t, logs)
			}
			if scenario.upstreamStatus == 0 {
				assert.Zero(t, calls.Load())
			} else {
				assert.Equal(t, int32(1), calls.Load())
			}
			tokenQuota := quota
			if strings.HasPrefix(scenario.path, "/pg/") {
				// 游乐场只扣用户钱包，不扣持久化 API 令牌额度。
				tokenQuota = 0
			}
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				require.NoError(collect, model.DB.First(token, token.Id).Error)
				require.NoError(collect, model.DB.First(user, user.Id).Error)
				assert.Equal(collect, 1000-tokenQuota, token.RemainQuota)
				assert.Equal(collect, tokenQuota, token.UsedQuota)
				assert.Equal(collect, 100000-quota, user.Quota)
				assert.Equal(collect, quota, user.UsedQuota)
			}, 3*time.Second, 10*time.Millisecond)
		})
	}
}
