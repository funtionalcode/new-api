package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

type adaptiveReasoningFixture struct {
	db             *gorm.DB
	parent, target *model.Channel
	calls          atomic.Int32
	response       atomic.Value
	status         atomic.Int32
	blockResponse  atomic.Bool
	states         chan map[string]any
}

func setupAdaptiveReasoning(t *testing.T) *adaptiveReasoningFixture {
	t.Helper()
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
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"jev-latest":0.021}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"jev-latest":0}`))
	service.InitHttpClient()
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "adaptive", Group: "default", Quota: 1000000, Status: 1}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 902, UserId: 901, Key: "adaptive-test", Name: "adaptive", Status: 1, UnlimitedQuota: true}).Error)
	f := &adaptiveReasoningFixture{db: db, states: make(chan map[string]any, 20)}
	f.response.Store(`{"model":"jev-latest","answers":{"effort":{"type":"choice","choice":"low"},"generations":{"type":"choice","choice":"2"}},"usage":{"input_tokens":1000,"output_tokens":0}}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		assert.Equal(t, "/v1/systemone", r.URL.Path)
		var request map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &request))
		f.states <- request
		if f.blockResponse.Load() {
			<-r.Context().Done()
			return
		}
		if status := f.status.Load(); status != 0 {
			w.WriteHeader(int(status))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, f.response.Load().(string))
	}))
	t.Cleanup(upstream.Close)
	f.parent = &model.Channel{Id: 903, Type: constant.ChannelTypeOpenAI, Status: 1, Models: "gpt-6-astra", Group: "default", Key: "main-test", BaseURL: common.GetPointer("https://upstream.invalid")}
	f.parent.SetSetting(dto.ChannelSettings{AdaptiveReasoning: &dto.AdaptiveReasoningConfig{Enabled: true, ChannelID: 904, Efforts: []string{"low", "high"}, MaxReuseGenerations: 2}})
	f.target = &model.Channel{Id: 904, Type: constant.ChannelTypeTypeSafe, Status: 1, Models: "jev-latest", Group: "default", Key: "evaluator-test", BaseURL: &upstream.URL, OpenUserIds: model.ChannelOpenUserIds{901}}
	require.NoError(t, db.Create(f.parent).Error)
	require.NoError(t, db.Create(f.target).Error)
	return f
}

func (f *adaptiveReasoningFixture) request(t *testing.T, input, session string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := &dto.OpenAIResponsesRequest{Model: "gpt-6-astra", Input: []byte(input), Reasoning: &dto.Reasoning{Effort: "high", Summary: "auto"}}
	body, err := common.Marshal(request)
	require.NoError(t, err)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	if session != "" {
		c.Request.Header.Set("X-New-Api-Conversation-Id", session)
	}
	c.Set("id", 901)
	cache, err := model.GetUserCache(901)
	require.NoError(t, err)
	cache.WriteContext(c)
	common.SetContextKey(c, constant.ContextKeyTokenId, 902)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "adaptive-test")
	common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, common.RequestIdKey, common.NewRequestId())
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, f.parent, "gpt-6-astra"))
	info := relaycommon.GenRelayInfoResponses(c, request)
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c, info
}

const adaptiveToolInput = `[{"role":"user","content":"Implement a parser"},{"type":"function_call","call_id":"call-1","name":"test","arguments":"{}"},{"type":"function_call_output","call_id":"call-1","output":"all tests pass"}]`

func TestAdaptiveReasoningAppliesDecisionWithoutChangingInputAndBillsOnlyEvaluation(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	for i := range 3 {
		c, info := f.request(t, adaptiveToolInput, t.Name())
		policy := service.RequestPolicy(c)
		prepareTypeSafeIntegration(c, info)
		assert.False(t, policy.Successful, "the evaluator must not mark the main request successful")
		require.Equal(t, "low", info.AdaptiveReasoningEffort, "%+v", info.TypeSafeResults)
		_, body, closer, apiErr := relay.PrepareResponsesRequest(c, info, info.Request.(*dto.OpenAIResponsesRequest))
		require.Nil(t, apiErr)
		wire, err := io.ReadAll(body)
		require.NoError(t, err)
		require.NoError(t, closer.Close())
		assert.Equal(t, "low", gjson.GetBytes(wire, "reasoning.effort").String())
		assert.Equal(t, "auto", gjson.GetBytes(wire, "reasoning.summary").String())
		assert.JSONEq(t, adaptiveToolInput, gjson.GetBytes(wire, "input").Raw)
		assert.Equal(t, "low", info.ReasoningEffort)
		assert.Equal(t, true, info.AdaptiveReasoningResult["applied"])
		publicResults := model.SanitizeTypeSafeResults(info.TypeSafeResults)
		require.Len(t, publicResults, 1)
		assert.Equal(t, "low", publicResults[0]["effort"])
		assert.Equal(t, true, publicResults[0]["applied"])
		assert.NotEmpty(t, publicResults[0]["answers"], "cached decisions must retain the evaluator answers")
		assert.NotContains(t, publicResults[0], "channel_id")
		assert.NotContains(t, publicResults[0], "decision_request_id")
		if i == 1 {
			assert.Equal(t, "cache", info.AdaptiveReasoningResult["source"])
		}
	}
	assert.EqualValues(t, 2, f.calls.Load())
	var user model.User
	require.NoError(t, f.db.First(&user, 901).Error)
	assert.EqualValues(t, 1000000-42, user.Quota)
	var logs []model.Log
	require.NoError(t, f.db.Where("channel_id = ?", 904).Find(&logs).Error)
	require.Len(t, logs, 2)
	assert.NotContains(t, logs[0].Other, "evaluator-test")
}

func TestAdaptiveReasoningReassessesOnNewInputFailureAndSessionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, input, session string
		token                int
	}{
		{"new user", `[{"role":"user","content":"Now debug a different problem"}]`, "same", 902},
		{"tool failure", strings.ReplaceAll(adaptiveToolInput, "all tests pass", "error: compilation failed"), "same", 902},
		{"nonzero exit", strings.ReplaceAll(adaptiveToolInput, "all tests pass", "Process exited with code 2"), "same", 902},
		{"another session", adaptiveToolInput, "other", 902},
		{"another token", adaptiveToolInput, "same", 903},
		{"no session", adaptiveToolInput, "", 902},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupAdaptiveReasoning(t)
			c, info := f.request(t, adaptiveToolInput, t.Name()+"same")
			prepareTypeSafeIntegration(c, info)
			session := tc.session
			if session != "" {
				session = t.Name() + session
			}
			c, info = f.request(t, tc.input, session)
			info.TokenId = tc.token
			prepareTypeSafeIntegration(c, info)
			assert.EqualValues(t, 2, f.calls.Load())
		})
	}
}

func TestAdaptiveReasoningNeverUsesCachedDecisionAfterPermissionRevocation(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	f.target.OpenUserIds = model.ChannelOpenUserIds{999}
	require.NoError(t, f.db.Save(f.target).Error)
	c, info = f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.Equal(t, "channel_unavailable_or_forbidden", info.TypeSafeResults[0]["reason"])
	assert.EqualValues(t, 1, f.calls.Load())
}

func TestAdaptiveReasoningFallsBackOnInvalidDecisionAndUnavailableEvaluator(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		status         int
	}{
		{"unknown effort", `{"model":"jev-latest","answers":{"effort":{"choice":"ultra"},"generations":{"choice":"2"}},"usage":{"input_tokens":1,"output_tokens":0}}`, 0},
		{"invalid window", `{"model":"jev-latest","answers":{"effort":{"choice":"low"},"generations":{"choice":"99"}},"usage":{"input_tokens":1,"output_tokens":0}}`, 0},
		{"failure", "", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupAdaptiveReasoning(t)
			f.response.Store(tc.response)
			f.status.Store(int32(tc.status))
			c, info := f.request(t, adaptiveToolInput, t.Name())
			prepareTypeSafeIntegration(c, info)
			assert.Empty(t, info.AdaptiveReasoningEffort)
			assert.Equal(t, "high", info.Request.(*dto.OpenAIResponsesRequest).Reasoning.Effort)
			assert.Equal(t, "error", info.TypeSafeResults[0]["status"])
			if tc.status == 0 {
				assert.Equal(t, "invalid_decision", info.TypeSafeResults[0]["reason"])
			}
			if tc.status != 0 {
				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					var user model.User
					require.NoError(collect, f.db.First(&user, 901).Error)
					assert.EqualValues(collect, 1000000, user.Quota)
				}, time.Second, time.Millisecond)
			}
		})
	}
}

func TestAdaptiveReasoningSettingsRejectUnsupportedAndConflictingConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*model.Channel, *dto.AdaptiveReasoningConfig)
	}{
		{"missing evaluator", func(_ *model.Channel, cfg *dto.AdaptiveReasoningConfig) { cfg.ChannelID = 0 }},
		{"unsupported effort", func(_ *model.Channel, cfg *dto.AdaptiveReasoningConfig) { cfg.Efforts = []string{"xhight"} }},
		{"unbounded timeout", func(_ *model.Channel, cfg *dto.AdaptiveReasoningConfig) { cfg.TimeoutMS = 30001 }},
		{"invalid window", func(_ *model.Channel, cfg *dto.AdaptiveReasoningConfig) { cfg.MaxReuseGenerations = 3 }},
		{"unsupported channel", func(ch *model.Channel, _ *dto.AdaptiveReasoningConfig) { ch.Type = constant.ChannelTypeGemini }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel := &model.Channel{Id: 1, Type: constant.ChannelTypeOpenAI}
			cfg := &dto.AdaptiveReasoningConfig{Enabled: true, ChannelID: 2}
			tc.change(channel, cfg)
			channel.SetSetting(dto.ChannelSettings{AdaptiveReasoning: cfg})
			require.Error(t, channel.ValidateSettings())
		})
	}
}

func TestAdaptiveReasoningBoundsContextAndExcludesPrivateReasoning(t *testing.T) {
	items := []any{map[string]any{"role": "user", "content": strings.Repeat("任务", 1000)}, map[string]any{"type": "reasoning", "encrypted_content": "private-canary", "summary": []any{map[string]any{"text": "Checking parser tests"}}}}
	for i := range 8 {
		items = append(items, map[string]any{"type": "function_call", "call_id": fmt.Sprint(i), "name": "test", "arguments": "{}"}, map[string]any{"type": "function_call_output", "call_id": fmt.Sprint(i), "output": strings.Repeat("ok", 1000)})
	}
	body, err := common.Marshal(map[string]any{"input": items})
	require.NoError(t, err)
	state := adaptiveRequestContext(body, 1200)
	encoded, err := common.Marshal(state)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "private-canary")
	assert.Len(t, state.Tools, 6)
	size := len([]rune(state.Request))
	for _, progress := range state.Progress {
		size += len([]rune(progress))
	}
	for _, tool := range state.Tools {
		size += len([]rune(tool.Name + tool.Arguments + tool.Output))
	}
	assert.LessOrEqual(t, size, 1200)
	assert.True(t, state.truncated)
}

func TestAdaptiveReasoningRedisReuseIsAtomicAndExpires(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldClient, oldEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() { common.RDB, common.RedisEnabled = oldClient, oldEnabled; require.NoError(t, client.Close()) })
	ctx := context.Background()
	key := t.Name()
	service.StoreAdaptiveReasoningDecision(ctx, key, service.AdaptiveReasoningDecision{Effort: "low", Generations: 10, Remaining: 9, UserHash: "user"})
	var wg sync.WaitGroup
	var consumed atomic.Int32
	for range 12 {
		wg.Go(func() {
			_, ok := service.TakeAdaptiveReasoningDecision(ctx, key, "user", "")
			if ok {
				consumed.Add(1)
			}
		})
	}
	wg.Wait()
	assert.EqualValues(t, 9, consumed.Load())
	service.StoreAdaptiveReasoningDecision(ctx, key, service.AdaptiveReasoningDecision{Effort: "low", Remaining: 1})
	server.FastForward(16 * time.Minute)
	_, ok := service.TakeAdaptiveReasoningDecision(ctx, key, "", "")
	assert.False(t, ok)
}

func TestAdaptiveReasoningChatWireOverridesFixedEffortOnly(t *testing.T) {
	info := &relaycommon.RelayInfo{AdaptiveReasoningEffort: "low"}
	data := []byte(`{"model":"gpt-6-astra","messages":[{"role":"user","content":"hello"}],"reasoning_effort":"high","prompt_cache_key":"stable-prefix"}`)
	out, err := relaycommon.ApplyAdaptiveReasoning(data, info)
	require.NoError(t, err)
	assert.Equal(t, "low", gjson.GetBytes(out, "reasoning_effort").String())
	assert.Equal(t, gjson.GetBytes(data, "messages").Raw, gjson.GetBytes(out, "messages").Raw)
	assert.Equal(t, "stable-prefix", gjson.GetBytes(out, "prompt_cache_key").String())
}

func TestAdaptiveReasoningWebSocketChangesEachGenerationAndReusesDecision(t *testing.T) {
	seen := make(chan string, 2)
	fixture := newResponsesWSBillingTest(t, `tier("request", fixed(0.002))`, func(ws *websocket.Conn, _ *http.Request) {
		for i := range 2 {
			_, payload, err := ws.ReadMessage()
			if !assert.NoError(t, err) {
				return
			}
			seen <- gjson.GetBytes(payload, "reasoning.effort").String()
			terminal := fmt.Sprintf(`{"type":"response.completed","response":{"id":"adaptive-%d","status":"completed","usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`, i)
			if !assert.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(terminal))) {
				return
			}
		}
		_, _, _ = ws.ReadMessage()
	})
	oldRatio, oldCompletion := ratio_setting.ModelRatio2JSONString(), ratio_setting.CompletionRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatio))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(oldCompletion))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"jev-latest":0.021}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"jev-latest":0}`))
	var evaluated atomic.Int32
	evaluator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		evaluated.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"jev-latest","answers":{"effort":{"choice":"low"},"generations":{"choice":"2"}},"usage":{"input_tokens":1,"output_tokens":0}}`)
	}))
	t.Cleanup(evaluator.Close)
	target := &model.Channel{Type: constant.ChannelTypeTypeSafe, Status: 1, Models: "jev-latest", Group: "default", Key: "test-evaluator", BaseURL: &evaluator.URL, OpenUserIds: model.ChannelOpenUserIds{fixture.user.Id}}
	require.NoError(t, model.DB.Create(target).Error)
	settings := fixture.channel.GetSetting()
	settings.AdaptiveReasoning = &dto.AdaptiveReasoningConfig{Enabled: true, ChannelID: target.Id, MaxReuseGenerations: 2}
	fixture.channel.SetSetting(settings)
	require.NoError(t, model.DB.Model(fixture.channel).Update("setting", fixture.channel.Setting).Error)
	for range 2 {
		payload := `{"type":"response.create","model":"ws-billing","input":` + adaptiveToolInput + `,"reasoning":{"effort":"high"}}`
		require.NoError(t, fixture.client.WriteMessage(websocket.TextMessage, []byte(payload)))
		event := readResponsesWSTestEvent(t, fixture.client)
		require.Equal(t, "response.completed", event["type"], "%v", event)
		assert.Equal(t, "low", <-seen)
	}
	fixture.closeAndWait(t)
	assert.EqualValues(t, 1, evaluated.Load())
}

func TestAdaptiveReasoningDisabledAndTokenRestrictionsDoNotCallEvaluator(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-6-astra": true})
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.Equal(t, "model_not_allowed", info.TypeSafeResults[0]["reason"])
	settings := f.parent.GetSetting()
	settings.AdaptiveReasoning.Enabled = false
	f.parent.SetSetting(settings)
	c, info = f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.TypeSafeResults)
	assert.Zero(t, f.calls.Load())
}

func TestAdaptiveReasoningCancelledEvaluationKeepsOriginalEffortAndRefunds(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.Equal(t, "high", info.Request.(*dto.OpenAIResponsesRequest).Reasoning.Effort)
	assert.Equal(t, "timeout_or_cancelled", info.TypeSafeResults[0]["reason"])
	assert.Zero(t, f.calls.Load())
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var user model.User
		require.NoError(collect, f.db.First(&user, 901).Error)
		assert.EqualValues(collect, 1000000, user.Quota)
	}, time.Second, time.Millisecond)
}

func TestAdaptiveReasoningTimeoutCancelsBlockedUpstreamAndRefunds(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	f.blockResponse.Store(true)
	settings := f.parent.GetSetting()
	settings.AdaptiveReasoning.TimeoutMS = 50
	f.parent.SetSetting(settings)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.Equal(t, "timeout_or_cancelled", info.TypeSafeResults[0]["reason"])
	assert.EqualValues(t, 1, f.calls.Load())
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var user model.User
		require.NoError(collect, f.db.First(&user, 901).Error)
		assert.EqualValues(collect, 1000000, user.Quota)
	}, time.Second, time.Millisecond)
}

func TestAdaptiveReasoningEmptyContextSkipsEvaluationAndPreservesEffort(t *testing.T) {
	for _, input := range []string{
		`[]`, `null`, `"  \n "`,
		`[{"role":"user","content":"  "}]`,
		`[{"type":"reasoning","encrypted_content":"private-canary","summary":[]}]`,
		`[{"type":"custom_tool_call_output","call_id":"image","output":[{"type":"input_image","image_url":"private-image-canary"}]}]`,
	} {
		t.Run(input, func(t *testing.T) {
			f := setupAdaptiveReasoning(t)
			c, info := f.request(t, input, t.Name())
			info.Request.(*dto.OpenAIResponsesRequest).Reasoning.Effort = "medium"
			info.SetReasoningEffort("medium")
			prepareTypeSafeIntegration(c, info)
			assert.Zero(t, f.calls.Load())
			assert.Empty(t, info.AdaptiveReasoningEffort)
			require.Len(t, info.TypeSafeResults, 1)
			assert.Equal(t, "skipped", info.TypeSafeResults[0]["status"])
			assert.Equal(t, "insufficient_context", info.TypeSafeResults[0]["reason"])
			_, body, closer, apiErr := relay.PrepareResponsesRequest(c, info, info.Request.(*dto.OpenAIResponsesRequest))
			require.Nil(t, apiErr)
			wire, err := io.ReadAll(body)
			require.NoError(t, err)
			require.NoError(t, closer.Close())
			assert.Equal(t, "medium", gjson.GetBytes(wire, "reasoning.effort").String())
			var user model.User
			require.NoError(t, f.db.First(&user, 901).Error)
			assert.EqualValues(t, 1000000, user.Quota)
		})
	}
}

func TestAdaptiveReasoningEmptyContextDoesNotConsumeOrReviveOldDecision(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	c, info = f.request(t, `[]`, t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.EqualValues(t, 1, f.calls.Load())
	c, info = f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.EqualValues(t, 2, f.calls.Load())
}

func TestAdaptiveReasoningCustomToolsReachEvaluatorAndFailuresReassess(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	input := `[{"role":"user","content":"Fix the parser"},{"type":"custom_tool_call","call_id":"patch-1","name":"apply_patch","input":"*** patch ***"},{"type":"custom_tool_call_output","call_id":"patch-1","output":[{"type":"input_text","text":"patch applied"},{"type":"input_image","image_url":"private-image-canary"}]}]`
	c, info := f.request(t, input, t.Name())
	prepareTypeSafeIntegration(c, info)
	require.EqualValues(t, 1, f.calls.Load())
	sent, err := common.Marshal(<-f.states)
	require.NoError(t, err)
	assert.Equal(t, "apply_patch", gjson.GetBytes(sent, "state.recent_tools.0.name").String())
	assert.Equal(t, "*** patch ***", gjson.GetBytes(sent, "state.recent_tools.0.arguments").String())
	assert.Equal(t, "patch applied", gjson.GetBytes(sent, "state.recent_tools.0.output").String())
	assert.NotContains(t, string(sent), "private-image-canary")
	c, info = f.request(t, strings.ReplaceAll(input, "patch applied", "error: patch failed"), t.Name())
	prepareTypeSafeIntegration(c, info)
	assert.EqualValues(t, 2, f.calls.Load())
}

func TestAdaptiveReasoningPartialContextCannotReuseOrStoreMultipleGenerations(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	c, info := f.request(t, adaptiveToolInput, t.Name())
	prepareTypeSafeIntegration(c, info)
	<-f.states
	for range 2 {
		c, info = f.request(t, `[{"type":"custom_tool_call_output","call_id":"patch-1","output":"error: context did not match"}]`, t.Name())
		prepareTypeSafeIntegration(c, info)
		require.Equal(t, "low", info.AdaptiveReasoningEffort)
		assert.Equal(t, 1, info.AdaptiveReasoningResult["generations"])
		assert.Equal(t, 0, info.AdaptiveReasoningResult["remaining"])
		assert.Equal(t, "evaluation", info.AdaptiveReasoningResult["source"])
		select {
		case sent := <-f.states:
			encoded, err := common.Marshal(sent)
			require.NoError(t, err)
			assert.True(t, gjson.GetBytes(encoded, "state.partial_context").Bool())
			assert.JSONEq(t, `{"1":"Keep this decision for 1 generations including the upcoming generation"}`, gjson.GetBytes(encoded, "questions.generations.criteria").Raw)
		default:
			t.Fatal("partial context must trigger a fresh assessment")
		}
	}
	assert.EqualValues(t, 3, f.calls.Load())
}

const adaptiveClaudeMessages = `[{"role":"user","content":[{"type":"text","text":"Fix parser","cache_control":{"type":"ephemeral"}}]},{"role":"assistant","content":[{"type":"thinking","thinking":"private-canary","signature":"signed-canary"},{"type":"text","text":"Checking tests"},{"type":"tool_use","id":"test-1","name":"Bash","input":{"command":"go test ./..."}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"test-1","content":[{"type":"text","text":"all tests pass"},{"type":"image","source":{"data":"image-canary"}}]}]}]`

func (f *adaptiveReasoningFixture) claudeRequest(t *testing.T, modelName, messages string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	c, _ := f.request(t, `[]`, t.Name())
	request := &dto.ClaudeRequest{Model: modelName, MaxTokens: common.GetPointer(uint(4096)), OutputConfig: []byte(`{"effort":"medium"}`)}
	session, err := common.Marshal(map[string]string{"session_id": t.Name()})
	require.NoError(t, err)
	request.Metadata, err = common.Marshal(map[string]string{"user_id": string(session)})
	require.NoError(t, err)
	require.NoError(t, common.UnmarshalJsonStr(messages, &request.Messages))
	encoded, err := common.Marshal(request)
	require.NoError(t, err)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(encoded))
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, f.parent, modelName))
	return c, relaycommon.GenRelayInfoClaude(c, request)
}

func TestAdaptiveReasoningClaudeNativeToolsReuseAndFailures(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	f.parent.Type = constant.ChannelTypeAnthropic
	require.NoError(t, f.parent.ValidateSettings())
	for i, messages := range []string{adaptiveClaudeMessages, adaptiveClaudeMessages, strings.ReplaceAll(adaptiveClaudeMessages, "all tests pass", "error: tests failed")} {
		c, info := f.claudeRequest(t, "claude-opus-5-5", messages)
		prepareTypeSafeIntegration(c, info)
		require.Equal(t, "low", info.AdaptiveReasoningEffort, "%+v", info.TypeSafeResults)
		if i == 1 {
			assert.Equal(t, "cache", info.AdaptiveReasoningResult["source"])
			continue
		}
		encoded, err := common.Marshal(<-f.states)
		require.NoError(t, err)
		assert.Equal(t, "Fix parser", gjson.GetBytes(encoded, "state.request").String())
		assert.Equal(t, "Bash", gjson.GetBytes(encoded, "state.recent_tools.0.name").String())
		assert.Contains(t, gjson.GetBytes(encoded, "state.recent_tools.0.arguments").String(), "go test")
		assert.False(t, gjson.GetBytes(encoded, "state.partial_context").Bool())
		assert.NotContains(t, string(encoded), "private-canary")
		assert.NotContains(t, string(encoded), "signed-canary")
		assert.NotContains(t, string(encoded), "image-canary")
	}
	assert.EqualValues(t, 2, f.calls.Load())
}

func TestAdaptiveReasoningClaudeModelCapabilitiesAndMappings(t *testing.T) {
	for _, modelName := range []string{"claude-opus-4-6", "claude-haiku-4-5-20251001"} {
		t.Run(modelName, func(t *testing.T) {
			f := setupAdaptiveReasoning(t)
			f.parent.Type = constant.ChannelTypeAnthropic
			settings := f.parent.GetSetting()
			settings.AdaptiveReasoning.Efforts = []string{"low", "xhigh", "max"}
			f.parent.SetSetting(settings)
			mapping, err := common.Marshal(map[string]string{"friendly-alias": modelName})
			require.NoError(t, err)
			f.parent.ModelMapping = common.GetPointer(string(mapping))
			c, info := f.claudeRequest(t, "friendly-alias", adaptiveClaudeMessages)
			prepareTypeSafeIntegration(c, info)
			if strings.Contains(modelName, "haiku") {
				assert.Zero(t, f.calls.Load())
				assert.Empty(t, info.AdaptiveReasoningEffort)
				assert.Equal(t, "unsupported_reasoning_model", info.TypeSafeResults[0]["reason"])
				return
			}
			require.Equal(t, "low", info.AdaptiveReasoningEffort, "%+v", info.TypeSafeResults)
			encoded, err := common.Marshal(<-f.states)
			require.NoError(t, err)
			choices := gjson.GetBytes(encoded, "questions.effort.criteria").Map()
			assert.Contains(t, choices, "max")
			assert.NotContains(t, choices, "xhigh")
		})
	}
}

func TestAdaptiveReasoningClaudeWritesNativeEffortForEveryInboundFormat(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	for _, tc := range []struct {
		format types.RelayFormat
		stream bool
	}{{types.RelayFormatClaude, false}, {types.RelayFormatClaude, true}, {types.RelayFormatOpenAI, false}, {types.RelayFormatOpenAIResponses, false}} {
		t.Run(fmt.Sprintf("%s_stream_%t", tc.format, tc.stream), func(t *testing.T) {
			format := tc.format
			f := setupAdaptiveReasoning(t)
			f.parent.Type = constant.ChannelTypeAnthropic
			settings := f.parent.GetSetting()
			settings.AdaptiveReasoning.Efforts = []string{"low", "max"}
			f.parent.SetSetting(settings)
			f.response.Store(`{"model":"jev-latest","answers":{"effort":{"choice":"max"},"generations":{"choice":"2"}},"usage":{"input_tokens":1,"output_tokens":0}}`)
			seen := make(chan []byte, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/messages", r.URL.Path)
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				seen <- body
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-5-5\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"msg-test","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2}}`)
			}))
			t.Cleanup(upstream.Close)
			f.parent.BaseURL = &upstream.URL
			c, info := f.claudeRequest(t, "claude-opus-5-5", adaptiveClaudeMessages)
			info.Request.(*dto.ClaudeRequest).Stream = common.GetPointer(tc.stream)
			info.IsStream = tc.stream
			if format != types.RelayFormatClaude {
				var request dto.Request
				if format == types.RelayFormatOpenAI {
					request = &dto.GeneralOpenAIRequest{Model: "claude-opus-5-5", Messages: []dto.Message{{Role: "user", Content: "Fix parser"}}}
					c.Request.URL.Path = "/v1/chat/completions"
				} else {
					request = &dto.OpenAIResponsesRequest{Model: "claude-opus-5-5", Input: []byte(`"Fix parser"`)}
					c.Request.URL.Path = "/v1/responses"
				}
				var err error
				info, err = relaycommon.GenRelayInfo(c, format, request, nil)
				require.NoError(t, err)
			}
			prepareTypeSafeIntegration(c, info)
			require.Equal(t, "max", info.AdaptiveReasoningEffort, "%+v", info.TypeSafeResults)
			if format == types.RelayFormatOpenAIResponses {
				_, body, closer, apiErr := relay.PrepareResponsesRequest(c, info, info.Request.(*dto.OpenAIResponsesRequest))
				require.Nil(t, apiErr)
				data, err := io.ReadAll(body)
				require.NoError(t, err)
				require.NoError(t, closer.Close())
				seen <- data
			} else if format == types.RelayFormatClaude {
				require.Nil(t, relay.ClaudeHelper(c, info))
			} else {
				require.Nil(t, relay.TextHelper(c, info))
			}
			data := <-seen
			assert.Equal(t, "max", gjson.GetBytes(data, "output_config.effort").String())
			assert.Equal(t, "adaptive", gjson.GetBytes(data, "thinking.type").String())
			assert.False(t, gjson.GetBytes(data, "reasoning_effort").Exists())
			assert.False(t, gjson.GetBytes(data, "thinking.budget_tokens").Exists())
			assert.Equal(t, true, info.AdaptiveReasoningResult["applied"])
			if format == types.RelayFormatClaude {
				assert.JSONEq(t, adaptiveClaudeMessages, gjson.GetBytes(data, "messages").Raw)
			}
		})
	}
}

func TestAdaptiveReasoningClaudeForcedToolsKeepOriginalRequest(t *testing.T) {
	f := setupAdaptiveReasoning(t)
	f.parent.Type = constant.ChannelTypeAnthropic
	c, info := f.claudeRequest(t, "claude-sonnet-4-6", adaptiveClaudeMessages)
	request := info.Request.(*dto.ClaudeRequest)
	request.ToolChoice = map[string]any{"type": "tool", "name": "Bash"}
	prepareTypeSafeIntegration(c, info)
	assert.Zero(t, f.calls.Load())
	assert.Empty(t, info.AdaptiveReasoningEffort)
	assert.Equal(t, "forced_tool_choice", info.TypeSafeResults[0]["reason"])
	assert.Equal(t, "medium", request.GetEfforts())
}

func TestAdaptiveReasoningClaudeOverridePreservesOutputOptionsAndTokenLimit(t *testing.T) {
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude, AdaptiveReasoningEffort: "max"}
	body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"task","cache_control":{"type":"ephemeral"}}],"max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":1024,"display":"summarized"},"output_config":{"effort":"medium","format":{"type":"json_schema","schema":{"type":"object"}}},"temperature":0.3,"top_p":0.5,"top_k":20}`)
	updated, err := relaycommon.ApplyAdaptiveReasoning(body, info)
	require.NoError(t, err)
	assert.Equal(t, "max", gjson.GetBytes(updated, "output_config.effort").String())
	assert.Equal(t, "adaptive", gjson.GetBytes(updated, "thinking.type").String())
	assert.Equal(t, "summarized", gjson.GetBytes(updated, "thinking.display").String())
	assert.EqualValues(t, 4096, gjson.GetBytes(updated, "max_tokens").Int())
	assert.Equal(t, gjson.GetBytes(body, "output_config.format").Raw, gjson.GetBytes(updated, "output_config.format").Raw)
	assert.Equal(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(updated, "messages").Raw)
	for _, field := range []string{"thinking.budget_tokens", "temperature", "top_p", "top_k"} {
		assert.False(t, gjson.GetBytes(updated, field).Exists(), field)
	}
}
