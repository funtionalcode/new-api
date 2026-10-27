package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const responsesImageTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg=="

const responsesImageTestRequest = `{"model":"image-tool-chat","input":"Draw a red panda","store":false,"tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen","parameters":{"type":"object","properties":{"prompt":{"type":"string"}}}}]},{"type":"image_generation"}],"tool_choice":{"type":"function","namespace":"image_gen","name":"imagegen"}}`

type responsesImageTestCall struct {
	body []byte
	auth string
}

type responsesImageTestFixture struct {
	user       *model.User
	token      *model.Token
	gateway    *httptest.Server
	chatCalls  chan responsesImageTestCall
	imageCalls chan responsesImageTestCall
	chat       http.HandlerFunc
	image      http.HandlerFunc
	nextChat   atomic.Int32
}

func newResponsesImageTestFixture(t *testing.T) *responsesImageTestFixture {
	t.Helper()
	previousMetrics := perf_metrics_setting.GetSetting().Enabled
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": "false"}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": fmt.Sprint(previousMetrics)}))
	})
	user, token := setupResponsesWSRequestTest(t)
	t.Setenv("RESPONSES_IMAGE_TOOL_MODEL", "")
	previousBatch, previousLogs, previousCount, previousQuota := common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit
	previousPrices, previousGroups := ratio_setting.ModelPrice2JSONString(), ratio_setting.GroupRatio2JSONString()
	previousRetries := common.RetryTimes
	previousTimeout := constant.StreamingTimeout
	common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit = false, true, false, 500000
	common.RetryTimes = 0
	constant.StreamingTimeout = 30
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"image-tool-chat":0.002,"grok-imagine-image-2.0":0.004,"gpt-image-2":0.004}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	t.Cleanup(func() {
		common.BatchUpdateEnabled, common.LogConsumeEnabled, constant.CountToken, common.QuotaPerUnit = previousBatch, previousLogs, previousCount, previousQuota
		common.RetryTimes = previousRetries
		constant.StreamingTimeout = previousTimeout
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousPrices))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroups))
	})
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	require.NoError(t, model.LOG_DB.AutoMigrate(&model.Log{}))
	require.NoError(t, model.DB.Model(user).Updates(map[string]any{"quota": 100000, "setting": `{"billing_preference":"wallet_only"}`}).Error)
	require.NoError(t, model.DB.Model(token).Update("remain_quota", 10000).Error)
	fixture := &responsesImageTestFixture{user: user, token: token, chatCalls: make(chan responsesImageTestCall, 10), imageCalls: make(chan responsesImageTestCall, 10)}
	fixture.chat = func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}
		fixture.chatCalls <- responsesImageTestCall{body: body, auth: r.Header.Get("Authorization")}
		assert.Equal(t, "/v1/responses", r.URL.Path)
		assert.True(t, gjson.GetBytes(body, "stream").Bool())
		assert.False(t, gjson.GetBytes(body, "parallel_tool_calls").Bool())
		for _, tool := range gjson.GetBytes(body, "tools").Array() {
			assert.NotEqual(t, "image_generation", tool.Get("type").String())
			assert.NotEqual(t, "image_gen", tool.Get("name").String())
		}
		call := fixture.nextChat.Add(1)
		id := fmt.Sprintf("resp_chat_%d", call)
		var output gin.H
		if gjson.GetBytes(body, "tool_choice").String() == "none" {
			output = gin.H{"type": "message", "id": "msg_result", "role": "assistant", "status": "completed", "content": []gin.H{{"type": "output_text", "text": "Here is the generated image.", "annotations": []any{}}}}
		} else {
			var name string
			for _, tool := range gjson.GetBytes(body, "tools").Array() {
				if strings.HasPrefix(tool.Get("name").String(), "newapi_image_generate_") {
					name = tool.Get("name").String()
				}
			}
			assert.NotEmpty(t, name)
			output = gin.H{"type": "function_call", "id": "fc_image", "call_id": "call_image", "name": name, "arguments": `{"prompt":"A red panda in watercolor"}`, "status": "completed"}
		}
		response := gin.H{"object": "response", "id": id, "model": "image-tool-chat", "status": "completed", "output": []gin.H{output}, "usage": gin.H{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}
		created := gin.H{"object": "response", "id": id, "model": "image-tool-chat", "status": "in_progress", "output": []any{}}
		writeResponsesImageTestEvents(t, w,
			gin.H{"type": "response.created", "response": created},
			gin.H{"type": "response.output_item.added", "output_index": 0, "item": output},
			gin.H{"type": "response.output_item.done", "output_index": 0, "item": output},
			gin.H{"type": "response.completed", "response": response},
		)
	}
	fixture.image = func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}
		fixture.imageCalls <- responsesImageTestCall{body: body, auth: r.Header.Get("Authorization")}
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":%q,"revised_prompt":"A red panda in watercolor"}]}`, responsesImageTestPNG)
	}
	for _, spec := range []struct {
		name, models, key string
		handler           http.HandlerFunc
	}{
		{"chat", "image-tool-chat", "chat-channel-secret", func(w http.ResponseWriter, r *http.Request) { fixture.chat(w, r) }},
		{"image", "grok-imagine-image-2.0,gpt-image-2", "image-channel-secret", func(w http.ResponseWriter, r *http.Request) { fixture.image(w, r) }},
	} {
		upstream := httptest.NewServer(spec.handler)
		t.Cleanup(upstream.Close)
		channel := &model.Channel{Name: spec.name, Key: spec.key, Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeOpenAI, Group: "default", Models: spec.models, BaseURL: &upstream.URL}
		require.NoError(t, channel.Insert())
		t.Cleanup(func() {
			require.NoError(t, model.DB.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error)
			require.NoError(t, model.DB.Delete(channel).Error)
		})
	}
	t.Cleanup(func() { require.NoError(t, model.LOG_DB.Where("token_id = ?", token.Id).Delete(&model.Log{}).Error) })
	engine := gin.New()
	engine.ForwardedByClientIP = false
	engine.Use(middleware.BodyStorageCleanup(), func(c *gin.Context) {
		c.Set(common.RequestIdKey, common.NewRequestId())
		common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	})
	engine.POST("/v1/responses", middleware.TokenAuth(), ResponsesImageToolMiddleware(engine), middleware.ModelRequestRateLimit(), middleware.RouteTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(), func(c *gin.Context) {
		RelayTaskPluginEndpoint(c, func(c *gin.Context) { Relay(c, types.RelayFormatOpenAIResponses) })
	})
	engine.POST("/v1/images/generations", middleware.TokenAuth(), middleware.ModelRequestRateLimit(), middleware.RouteTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(), func(c *gin.Context) {
		RelayTaskPluginEndpoint(c, func(c *gin.Context) { Relay(c, types.RelayFormatOpenAIImage) })
	})
	engine.GET("/v1/responses", middleware.TokenAuth(), ResponsesWebSocketWithImageTools(engine))
	fixture.gateway = httptest.NewServer(engine)
	t.Cleanup(fixture.gateway.Close)
	return fixture
}

func writeResponsesImageTestEvents(t *testing.T, w http.ResponseWriter, events ...gin.H) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	for index, event := range events {
		event["sequence_number"] = index
		body, err := common.Marshal(event)
		if !assert.NoError(t, err) {
			return
		}
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], body)
	}
}

func (fixture *responsesImageTestFixture) request(t *testing.T, body []byte) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, fixture.gateway.URL+"/v1/responses", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer sk-"+fixture.token.Key)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, data
}

func (fixture *responsesImageTestFixture) assertAccounting(t *testing.T, quotas ...int) {
	t.Helper()
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("token_id = ? AND type = ?", fixture.token.Id, model.LogTypeConsume).Order("id").Find(&logs).Error)
	require.Len(t, logs, len(quotas))
	var total int
	for index, quota := range quotas {
		assert.Equal(t, quota, logs[index].Quota)
		assert.Equal(t, fixture.user.Id, logs[index].UserId)
		total += quota
	}
	require.NoError(t, model.DB.First(fixture.token, fixture.token.Id).Error)
	require.NoError(t, model.DB.First(fixture.user, fixture.user.Id).Error)
	assert.Equal(t, 10000-total, fixture.token.RemainQuota)
	assert.Equal(t, total, fixture.token.UsedQuota)
	assert.Equal(t, 100000-total, fixture.user.Quota)
	assert.Equal(t, total, fixture.user.UsedQuota)
}

func TestResponsesImageToolRoutesWithCallerToken(t *testing.T) {
	for _, transport := range []string{"json", "sse", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			fixture := newResponsesImageTestFixture(t)
			setting.ModelRequestRateLimitEnabled, setting.ModelRequestRateLimitDurationMinutes = true, 1
			setting.ModelRequestRateLimitCount, setting.ModelRequestRateLimitSuccessCount = 3, 0
			body := []byte(responsesImageTestRequest)
			var response []byte
			var events [][]byte
			if transport == "websocket" {
				client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(fixture.gateway.URL, "http")+"/v1/responses", http.Header{"Authorization": {"Bearer sk-" + fixture.token.Key}})
				require.NoError(t, err)
				defer client.Close()
				body, err = sjson.SetBytes(body, "type", "response.create")
				require.NoError(t, err)
				require.NoError(t, client.WriteMessage(websocket.TextMessage, body))
				for {
					require.NoError(t, client.SetReadDeadline(time.Now().Add(5*time.Second)))
					_, event, err := client.ReadMessage()
					require.NoError(t, err)
					events = append(events, event)
					if isResponsesImageToolTerminal(gjson.GetBytes(event, "type").String()) || gjson.GetBytes(event, "type").String() == "error" {
						response = []byte(gjson.GetBytes(event, "response").Raw)
						break
					}
				}
			} else {
				var err error
				body, err = sjson.SetBytes(body, "stream", transport == "sse")
				require.NoError(t, err)
				status, data := fixture.request(t, body)
				require.Equal(t, http.StatusOK, status, string(data))
				if transport == "sse" {
					for line := range strings.SplitSeq(string(data), "\n") {
						if payload, found := strings.CutPrefix(line, "data: "); found {
							events = append(events, []byte(payload))
							if gjson.Get(payload, "type").String() == "response.completed" {
								response = []byte(gjson.Get(payload, "response").Raw)
							}
						}
					}
				} else {
					response = data
				}
			}
			require.NotEmpty(t, response, "%s", events)
			assert.Equal(t, "resp_chat_2", gjson.GetBytes(response, "id").String())
			assert.Equal(t, "completed", gjson.GetBytes(response, "status").String())
			assert.Equal(t, "image_generation_call", gjson.GetBytes(response, "output.0.type").String())
			assert.Equal(t, responsesImageTestPNG, gjson.GetBytes(response, "output.0.result").String())
			assert.Equal(t, "png", gjson.GetBytes(response, "output.0.output_format").String())
			assert.Equal(t, "message", gjson.GetBytes(response, "output.1.type").String())
			assert.Equal(t, int64(30), gjson.GetBytes(response, "usage.total_tokens").Int())
			assert.JSONEq(t, gjson.Get(responsesImageTestRequest, "tools").Raw, gjson.GetBytes(response, "tools").Raw)
			for index, event := range events {
				assert.Equal(t, int64(index), gjson.GetBytes(event, "sequence_number").Int())
				assert.NotContains(t, string(event), "newapi_image_generate_")
				if gjson.GetBytes(event, "item.type").String() == "message" {
					assert.Equal(t, int64(1), gjson.GetBytes(event, "output_index").Int())
				}
			}
			require.Len(t, fixture.chatCalls, 2)
			first, last := <-fixture.chatCalls, <-fixture.chatCalls
			assert.Equal(t, "Bearer chat-channel-secret", first.auth)
			assert.Equal(t, "Bearer chat-channel-secret", last.auth)
			assert.Equal(t, "function_call_output", gjson.GetBytes(last.body, "input.2.type").String())
			assert.False(t, gjson.GetBytes(last.body, "store").Bool())
			require.Len(t, fixture.imageCalls, 1)
			image := <-fixture.imageCalls
			assert.Equal(t, "Bearer image-channel-secret", image.auth)
			assert.JSONEq(t, `{"model":"grok-imagine-image-2.0","prompt":"A red panda in watercolor","n":1,"aspect_ratio":"1:1","resolution":"1k","quality":"auto","response_format":"b64_json"}`, string(image.body))
			fixture.assertAccounting(t, 1000, 2000, 1000)
		})
	}
}

func TestResponsesImageToolPassesThroughOrdinaryChat(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			fixture := newResponsesImageTestFixture(t)
			chat := fixture.chat
			fixture.chat = func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if !assert.NoError(t, err) {
					return
				}
				body, err = sjson.SetBytes(body, "tool_choice", "none")
				if !assert.NoError(t, err) {
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				chat(w, r)
			}
			body, err := sjson.SetBytes([]byte(responsesImageTestRequest), "tool_choice", "auto")
			require.NoError(t, err)
			body, err = sjson.SetBytes(body, "stream", streaming)
			require.NoError(t, err)
			status, data := fixture.request(t, body)
			require.Equal(t, http.StatusOK, status, string(data))
			response := data
			if streaming {
				response = nil
				for line := range strings.SplitSeq(string(data), "\n") {
					if payload, found := strings.CutPrefix(line, "data: "); found && gjson.Get(payload, "type").String() == "response.completed" {
						response = []byte(gjson.Get(payload, "response").Raw)
					}
				}
			}
			assert.Equal(t, "resp_chat_1", gjson.GetBytes(response, "id").String())
			assert.Equal(t, "message", gjson.GetBytes(response, "output.0.type").String())
			assert.Equal(t, "auto", gjson.GetBytes(response, "tool_choice").String())
			assert.NotContains(t, string(data), "newapi_image_generate_")
			assert.Len(t, fixture.chatCalls, 1)
			assert.Empty(t, fixture.imageCalls)
			fixture.assertAccounting(t, 1000)
		})
	}
}

func TestResponsesImageToolEnforcesCallerPolicy(t *testing.T) {
	for _, policy := range []string{"revoked", "expired", "ip", "group", "model", "quota", "rate", "revoked-before-image", "disabled-user-before-image"} {
		t.Run(policy, func(t *testing.T) {
			fixture := newResponsesImageTestFixture(t)
			updates := map[string]any{}
			wantStatus := http.StatusUnauthorized
			switch policy {
			case "revoked":
				updates["status"] = common.TokenStatusDisabled
			case "expired":
				updates["expired_time"] = 1
			case "ip":
				updates["allow_ips"] = "203.0.113.8"
				wantStatus = http.StatusForbidden
			case "group":
				updates["group"] = "image-tool-forbidden-group"
				wantStatus = http.StatusForbidden
			case "model":
				updates["model_limits_enabled"], updates["model_limits"] = true, "image-tool-chat"
				wantStatus = http.StatusForbidden
			case "quota":
				updates["remain_quota"] = 1500
				wantStatus = http.StatusForbidden
			case "rate":
				setting.ModelRequestRateLimitEnabled, setting.ModelRequestRateLimitDurationMinutes = true, 1
				setting.ModelRequestRateLimitCount, setting.ModelRequestRateLimitSuccessCount = 1, 0
				wantStatus = http.StatusTooManyRequests
			case "revoked-before-image", "disabled-user-before-image":
				chat := fixture.chat
				fixture.chat = func(w http.ResponseWriter, r *http.Request) {
					chat(w, r)
					if policy == "revoked-before-image" {
						assert.NoError(t, model.DB.Model(fixture.token).Update("status", common.TokenStatusDisabled).Error)
					} else {
						assert.NoError(t, model.DB.Model(fixture.user).Update("status", common.UserStatusDisabled).Error)
					}
				}
				if policy == "disabled-user-before-image" {
					wantStatus = http.StatusForbidden
				}
			}
			require.NoError(t, model.DB.Model(fixture.token).Updates(updates).Error)
			status, data := fixture.request(t, []byte(responsesImageTestRequest))
			assert.Equal(t, wantStatus, status, string(data))
			assert.True(t, gjson.GetBytes(data, "error").IsObject(), string(data))
			assert.Empty(t, fixture.imageCalls)
			if policy == "model" || policy == "quota" || policy == "rate" || policy == "revoked-before-image" || policy == "disabled-user-before-image" {
				assert.Len(t, fixture.chatCalls, 1)
			} else {
				assert.Empty(t, fixture.chatCalls)
			}
		})
	}
}

func TestResponsesImageToolDoesNotRetryGeneration(t *testing.T) {
	fixture := newResponsesImageTestFixture(t)
	common.RetryTimes = 2
	var submissions atomic.Int32
	fixture.image = func(w http.ResponseWriter, _ *http.Request) {
		submissions.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":{"message":"provider failed after accepting the request","type":"api_error"}}`)
	}
	status, _ := fixture.request(t, []byte(responsesImageTestRequest))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, int32(1), submissions.Load())
	fixture.assertAccounting(t, 1000)
}

func TestResponsesImageToolRejectsUnsafeCallsBeforeSubmission(t *testing.T) {
	for _, scenario := range []string{"disabled", "excluded", "multiple", "mixed", "incomplete", "invalid-arguments"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := newResponsesImageTestFixture(t)
			fixture.chat = func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if !assert.NoError(t, err) {
					return
				}
				call := gin.H{"id": "fc_image", "type": "function_call", "call_id": "call_image", "name": gjson.GetBytes(body, "tools.0.name").String(), "arguments": `{"prompt":"panda"}`, "status": "completed"}
				output := []gin.H{call}
				status := "completed"
				switch scenario {
				case "multiple":
					output = append(output, call)
				case "mixed":
					output = append(output, gin.H{"id": "fc_client", "type": "function_call", "name": "client_tool", "call_id": "client_call", "arguments": "{}"})
				case "incomplete":
					status = "incomplete"
				case "invalid-arguments":
					call["arguments"] = `{"prompt":" "}`
				}
				writeResponsesImageTestEvents(t, w, gin.H{"type": "response." + status, "response": gin.H{"id": "resp_unsafe", "object": "response", "status": status, "output": output, "usage": gin.H{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}})
			}
			body := []byte(responsesImageTestRequest)
			var err error
			if scenario == "disabled" {
				body, err = sjson.SetBytes(body, "tool_choice", "none")
			} else if scenario == "excluded" {
				body, err = sjson.SetRawBytes(body, "tool_choice", []byte(`{"type":"allowed_tools","mode":"auto","tools":[]}`))
			}
			require.NoError(t, err)
			status, data := fixture.request(t, body)
			assert.GreaterOrEqual(t, status, http.StatusBadRequest, string(data))
			assert.True(t, gjson.GetBytes(data, "error").IsObject())
			assert.Empty(t, fixture.imageCalls)
		})
	}
}

func TestResponsesImageToolAcceptsOnlyCompletedGPTImage(t *testing.T) {
	fixture := newResponsesImageTestFixture(t)
	fixture.image = func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			return
		}
		fixture.imageCalls <- responsesImageTestCall{body: body, auth: r.Header.Get("Authorization")}
		writeResponsesImageTestEvents(t, w,
			gin.H{"type": "image_generation.partial_image", "b64_json": "not-a-complete-image", "partial_image_index": 0},
			gin.H{"type": "image_generation.completed", "b64_json": responsesImageTestPNG},
		)
	}
	body, err := sjson.SetBytes([]byte(responsesImageTestRequest), "tools.1.model", "gpt-image-2")
	require.NoError(t, err)
	status, data := fixture.request(t, body)
	require.Equal(t, http.StatusOK, status, string(data))
	assert.Equal(t, responsesImageTestPNG, gjson.GetBytes(data, "output.0.result").String())
	require.Len(t, fixture.imageCalls, 1)
	image := <-fixture.imageCalls
	assert.Equal(t, "gpt-image-2", gjson.GetBytes(image.body, "model").String())
	assert.True(t, gjson.GetBytes(image.body, "stream").Bool())
	assert.Equal(t, int64(1), gjson.GetBytes(image.body, "partial_images").Int())
	fixture.assertAccounting(t, 1000, 2000, 1000)
}

func TestResponsesImageToolPreservesPaidImageOnContinuationFailure(t *testing.T) {
	for _, failure := range []string{"http-error", "truncated-stream", "repeated-image"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newResponsesImageTestFixture(t)
			chat := fixture.chat
			fixture.chat = func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if !assert.NoError(t, err) {
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				if gjson.GetBytes(body, "tool_choice").String() != "none" {
					chat(w, r)
					return
				}
				if failure == "http-error" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"continuation rejected"}}`)
					return
				}
				writeResponsesImageTestEvents(t, w, gin.H{"type": "response.created", "response": gin.H{"id": "resp_partial", "object": "response", "status": "in_progress", "output": []any{}}})
				if failure == "repeated-image" {
					name := gjson.GetBytes(body, "tools.0.name").String()
					writeResponsesImageTestEvents(t, w, gin.H{"type": "response.output_item.added", "item": gin.H{"type": "function_call", "name": name}})
				}
			}
			status, data := fixture.request(t, []byte(responsesImageTestRequest))
			require.Equal(t, http.StatusOK, status, string(data))
			assert.Equal(t, "failed", gjson.GetBytes(data, "status").String())
			assert.Equal(t, responsesImageTestPNG, gjson.GetBytes(data, "output.0.result").String())
			assert.NotEmpty(t, gjson.GetBytes(data, "error.message").String())
			assert.Len(t, fixture.imageCalls, 1)
			if failure == "http-error" {
				fixture.assertAccounting(t, 1000, 2000)
			}
		})
	}
}

func TestResponsesImageToolSelectionAndImageProtocols(t *testing.T) {
	for _, selection := range []string{`"none"`, `{"type":"function","name":"another_tool"}`, `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"another_tool"}]}`} {
		tool, err := prepareResponsesImageTool([]byte(strings.Replace(responsesImageTestRequest, `{"type":"function","namespace":"image_gen","name":"imagegen"}`, selection, 1)))
		require.NoError(t, err)
		assert.False(t, tool.allowed)
	}
	assert.False(t, hasResponsesImageTool([]byte(`{"tools":[{"type":"image_generation","action":"edit"}]}`)), "已有原生 hosted-only 请求保持原路径")
	assert.True(t, hasResponsesImageTool([]byte(`{"tools":[{"type":"function","name":"image_gen.imagegen"}]}`)))
	tool, err := prepareResponsesImageTool([]byte(responsesImageTestRequest))
	require.NoError(t, err)
	assert.True(t, tool.allowed)
	assert.Equal(t, tool.name, gjson.GetBytes(tool.request, "tool_choice.name").String())
	tool.settings = gjson.Parse(`{"model":"gpt-image-2"}`)
	request, err := tool.imageRequest(`{"prompt":"hello"}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gpt-image-2","prompt":"hello","n":1,"size":"1024x1024","quality":"high","output_format":"png","stream":true,"partial_images":1}`, string(request))
	_, err = tool.imageRequest(`{"prompt":" "}`)
	require.Error(t, err)
	exchange := &responsesImageToolExchange{events: [][]byte{[]byte(`{"type":"image_generation.partial_image","b64_json":"incomplete"}`)}}
	_, _, err = exchange.imageResult(context.Background())
	require.Error(t, err)
	exchange.events = append(exchange.events, []byte(fmt.Sprintf(`{"type":"image_generation.completed","b64_json":%q}`, responsesImageTestPNG)))
	result, format, err := exchange.imageResult(context.Background())
	require.NoError(t, err)
	assert.Equal(t, responsesImageTestPNG, result)
	assert.Equal(t, "png", format)
}

func TestResponsesImageToolSystemModelSetting(t *testing.T) {
	setupResponsesWSRequestTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	const optionKey = "global.responses_image_tool_model"
	defaults := config.GlobalConfig.ExportAllConfigs()
	previousModel := defaults[optionKey]
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = defaults
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{optionKey: previousModel}))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})

	router := gin.New()
	router.GET("/api/option/", GetOptions)
	router.PUT("/api/option/", UpdateOption)
	options := httptest.NewRecorder()
	router.ServeHTTP(options, httptest.NewRequest(http.MethodGet, "/api/option/", nil))
	require.Equal(t, http.StatusOK, options.Code)
	require.True(t, gjson.GetBytes(options.Body.Bytes(), `data.#(key=="global.responses_image_tool_model").value`).Exists(), "系统配置接口必须返回图片工具模型设置")

	for _, tc := range []struct {
		name        string
		systemModel string
		envModel    string
		toolModel   string
		want        string
	}{
		{name: "default", want: "grok-imagine-image-2.0"},
		{name: "environment fallback", envModel: " grok-imagine-image-custom ", want: "grok-imagine-image-custom"},
		{name: "system overrides environment", systemModel: " gpt-image-2 ", envModel: "grok-imagine-image-2.0", want: "gpt-image-2"},
		{name: "saved change applies immediately", systemModel: "grok-imagine-image-new", envModel: "gpt-image-2", want: "grok-imagine-image-new"},
		{name: "tool overrides system", systemModel: "grok-imagine-image-2.0", envModel: "grok-imagine-image-new", toolModel: "gpt-image-2", want: "gpt-image-2"},
		{name: "clearing restores environment", envModel: "gpt-image-2", want: "gpt-image-2"},
		{name: "blank system uses default", systemModel: " \t ", want: "grok-imagine-image-2.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RESPONSES_IMAGE_TOOL_MODEL", tc.envModel)
			body, err := common.Marshal(gin.H{"key": optionKey, "value": tc.systemModel})
			require.NoError(t, err)
			saved := httptest.NewRecorder()
			router.ServeHTTP(saved, httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body)))
			require.Equal(t, http.StatusOK, saved.Code)
			require.True(t, gjson.GetBytes(saved.Body.Bytes(), "success").Bool(), saved.Body.String())
			var persisted model.Option
			require.NoError(t, model.DB.Where(&model.Option{Key: optionKey}).First(&persisted).Error)
			assert.Equal(t, tc.systemModel, persisted.Value)

			options := httptest.NewRecorder()
			router.ServeHTTP(options, httptest.NewRequest(http.MethodGet, "/api/option/", nil))
			assert.Equal(t, tc.systemModel, gjson.GetBytes(options.Body.Bytes(), `data.#(key=="global.responses_image_tool_model").value`).String())
			settings, err := common.Marshal(gin.H{"model": tc.toolModel})
			require.NoError(t, err)
			tool := &responsesImageTool{settings: gjson.ParseBytes(settings)}
			image, err := tool.imageRequest(`{"prompt":"Draw a red panda"}`)
			require.NoError(t, err)
			assert.Equal(t, tc.want, gjson.GetBytes(image, "model").String())
		})
	}
}

func TestResponsesImageToolDownloadsWithoutCredentials(t *testing.T) {
	if service.GetSSRFProtectedHTTPClient() == nil {
		service.InitHttpClient()
	}
	fetch := system_setting.GetFetchSetting()
	previous := *fetch
	t.Cleanup(func() { *fetch = previous })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Cookie"))
		payload, err := base64.StdEncoding.DecodeString(responsesImageTestPNG)
		if assert.NoError(t, err) {
			_, _ = w.Write(payload)
		}
	}))
	defer server.Close()
	address, err := url.Parse(server.URL)
	require.NoError(t, err)
	fetch.EnableSSRFProtection, fetch.AllowPrivateIp, fetch.AllowedPorts = true, true, []string{address.Port()}
	exchange := &responsesImageToolExchange{}
	_, err = fmt.Fprintf(&exchange.body, `{"data":[{"url":%q}]}`, server.URL)
	require.NoError(t, err)
	result, format, err := exchange.imageResult(context.Background())
	require.NoError(t, err)
	assert.Equal(t, responsesImageTestPNG, result)
	assert.Equal(t, "png", format)
	fetch.AllowPrivateIp = false
	_, _, err = exchange.imageResult(context.Background())
	require.ErrorContains(t, err, "fetch policy")
}
