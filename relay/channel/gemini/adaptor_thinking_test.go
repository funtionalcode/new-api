package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

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
