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
