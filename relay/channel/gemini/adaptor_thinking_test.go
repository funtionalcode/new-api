package gemini

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	hostreasoning "github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeminiForwardingPreservesRealEffortTailModelIDs(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := *settings
	t.Cleanup(func() { *settings = original })
	settings.PassThroughRequestEnabled = false
	settings.ThinkingModelBlacklist = nil
	settings.EffortTailModelIDs = append(append([]string(nil), settings.EffortTailModelIDs...), "gemini-custom-high")

	for _, tc := range []struct {
		name            string
		model           string
		mapping         string
		requestEffort   string
		wantModel       string
		wantEffort      string
		wantBillingName string
	}{
		{
			name:      "完整模型名称",
			model:     "gemini-3.8-flash-high",
			wantModel: "gemini-3.8-flash-high",
		},
		{
			name:          "模型映射保留请求中的思考强度",
			model:         "client-model",
			mapping:       `{"client-model":"gemini-3.8-flash-high"}`,
			requestEffort: "minimal",
			wantModel:     "gemini-3.8-flash-high",
			wantEffort:    "low",
		},
		{
			name:      "带命名空间的完整模型名称",
			model:     "gemini-3.8-flash-high",
			mapping:   `{"gemini-3.8-flash-high":"vendor/gemini-3.8-flash-high"}`,
			wantModel: "vendor/gemini-3.8-flash-high",
		},
		{
			name:            "完整模型仍支持显式思考参数",
			model:           "gemini-3.8-flash-high@effort:low",
			wantModel:       "gemini-3.8-flash-high",
			wantEffort:      "low",
			wantBillingName: "gemini-3.8-flash-high@effort:low@thinking:on",
		},
		{
			name:      "管理员配置的完整模型名称",
			model:     "gemini-custom-high",
			wantModel: "gemini-custom-high",
		},
		{
			name:            "普通思考别名继续生效",
			model:           "gemini-3-flash-preview-high",
			wantModel:       "gemini-3-flash-preview",
			wantEffort:      "high",
			wantBillingName: "gemini-3-flash-preview@effort:high@thinking:on",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatGemini} {
				t.Run(string(format), func(t *testing.T) {
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
					c.Set("model_mapping", tc.mapping)
					var request dto.Request
					switch format {
					case types.RelayFormatOpenAI:
						request = &dto.GeneralOpenAIRequest{
							Model: tc.model, Messages: []dto.Message{{Role: "user", Content: "hello"}}, ReasoningEffort: tc.requestEffort,
						}
					case types.RelayFormatOpenAIResponses:
						var responsesRequest dto.OpenAIResponsesRequest
						require.NoError(t, common.UnmarshalJsonStr(`{"input":"hello"}`, &responsesRequest))
						responsesRequest.Model = tc.model
						if tc.requestEffort != "" {
							responsesRequest.Reasoning = &dto.Reasoning{Effort: tc.requestEffort}
						}
						request = &responsesRequest
					case types.RelayFormatGemini:
						var geminiRequest dto.GeminiChatRequest
						require.NoError(t, common.UnmarshalJsonStr(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`, &geminiRequest))
						geminiRequest.SetModelName(tc.model)
						if tc.requestEffort != "" {
							require.NoError(t, common.UnmarshalJsonStr(`{"thinkingConfig":{"thinkingLevel":"`+tc.requestEffort+`"}}`, &geminiRequest.GenerationConfig))
						}
						request = &geminiRequest
					}
					info := &relaycommon.RelayInfo{
						RelayFormat: format, OriginModelName: tc.model, Request: request, IsStream: true,
						ChannelMeta: &relaycommon.ChannelMeta{
							UpstreamModelName: tc.model, ChannelBaseUrl: "https://gemini.example.com",
						},
					}
					require.NoError(t, helper.ModelMappedHelper(c, info, request))
					require.NoError(t, helper.ApplyReasoningModelSuffix(c, info, request))
					adaptor := &Adaptor{}
					var converted any
					var err error
					switch r := request.(type) {
					case *dto.GeneralOpenAIRequest:
						converted, err = adaptor.ConvertOpenAIRequest(c, info, r)
					case *dto.OpenAIResponsesRequest:
						converted, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *r)
					case *dto.GeminiChatRequest:
						converted, err = adaptor.ConvertGeminiRequest(c, info, r)
					}
					require.NoError(t, err)
					body, err := common.Marshal(converted)
					require.NoError(t, err)
					assert.Equal(t, tc.wantEffort, gjson.GetBytes(body, "generationConfig.thinkingConfig.thinkingLevel").String())
					url, err := adaptor.GetRequestURL(info)
					require.NoError(t, err)
					assert.Equal(t, "https://gemini.example.com/v1beta/models/"+tc.wantModel+":streamGenerateContent?alt=sse", url)
					if tc.mapping == "" {
						assert.Equal(t, tc.wantModel, hostreasoning.BaseModelName(tc.model))
						billingNames := hostreasoning.CanonicalBillingModelNames(tc.model)
						if tc.wantBillingName == "" {
							assert.Empty(t, billingNames)
						} else {
							require.NotEmpty(t, billingNames)
							assert.Equal(t, tc.wantBillingName, billingNames[0])
						}
					}
				})
			}
		})
	}
}

func TestGeminiAdaptorNormalizesMinimalForMappedFlashModel(t *testing.T) {
	for _, format := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses} {
		t.Run(string(format), func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayFormat:     format,
				OriginModelName: "client-model",
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "gemini-3.8-flash-high",
					ChannelBaseUrl:    "https://gemini.example.com",
					IsModelMapped:     true,
				},
			}
			adaptor := &Adaptor{}
			var converted any
			var err error
			switch format {
			case types.RelayFormatOpenAI:
				converted, err = adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
					Model:           "client-model",
					Messages:        []dto.Message{{Role: "user", Content: "hello"}},
					ReasoningEffort: "minimal",
				})
			case types.RelayFormatOpenAIResponses:
				var request dto.OpenAIResponsesRequest
				require.NoError(t, common.UnmarshalJsonStr(`{"model":"client-model","input":"hello","reasoning":{"effort":"minimal"}}`, &request))
				converted, err = adaptor.ConvertOpenAIResponsesRequest(nil, info, request)
			}
			require.NoError(t, err)
			require.IsType(t, &dto.GeminiChatRequest{}, converted)
			body, err := common.Marshal(converted)
			require.NoError(t, err)
			assert.Equal(t, "low", gjson.GetBytes(body, "generationConfig.thinkingConfig.thinkingLevel").String())
			assert.False(t, gjson.GetBytes(body, "generationConfig.thinkingConfig.thinkingBudget").Exists())
			assert.Equal(t, "low", info.GetReasoningEffort())
			url, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, "https://gemini.example.com/v1beta/models/gemini-3.8-flash-high:generateContent", url)
		})
	}
}

func TestGeminiAdaptorNormalizesNativeMinimalThinkingConfig(t *testing.T) {
	for _, tt := range []struct {
		name   string
		format types.RelayFormat
		effort string
	}{
		{"extra_body", types.RelayFormatOpenAI, ""},
		{"extra_body_with_reasoning_effort", types.RelayFormatOpenAI, "minimal"},
		{"native_gemini", types.RelayFormatGemini, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayFormat:     tt.format,
				OriginModelName: "gemini-3.8-flash-high",
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gemini-3.8-flash-high"},
			}
			var converted any
			var err error
			if tt.format == types.RelayFormatOpenAI {
				converted, err = (&Adaptor{}).ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
					Model:           "gemini-3.8-flash-high",
					Messages:        []dto.Message{{Role: "user", Content: "hello"}},
					ReasoningEffort: tt.effort,
					ExtraBody:       []byte(`{"google":{"thinking_config":{"thinking_level":"minimal","include_thoughts":false}}}`),
				})
			} else {
				var request dto.GeminiChatRequest
				require.NoError(t, common.UnmarshalJsonStr(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"generationConfig":{"thinkingConfig":{"thinkingLevel":"minimal","includeThoughts":false}}}`, &request))
				converted, err = (&Adaptor{}).ConvertGeminiRequest(nil, info, &request)
			}
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)
			assert.Equal(t, "low", gjson.GetBytes(body, "generationConfig.thinkingConfig.thinkingLevel").String())
			assert.Equal(t, "false", gjson.GetBytes(body, "generationConfig.thinkingConfig.includeThoughts").Raw)
			assert.False(t, gjson.GetBytes(body, "generationConfig.thinkingConfig.thinkingBudget").Exists())
			assert.Equal(t, "low", info.GetReasoningEffort())
		})
	}
}
