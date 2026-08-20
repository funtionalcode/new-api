package relay

import (
	"net/http/httptest"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResponsesWebsocketUpstreamPayloadRemovesXAITransportFields(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"grok-4.5","input":[],"stream":true,"background":true}`)
	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, appcommon.Unmarshal(payload, request))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		IsStream:       true,
		IsWebsocket:    true,
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           appconstant.APITypeXai,
			ChannelType:       appconstant.ChannelTypeXai,
			ChannelBaseUrl:    "https://api.x.ai",
			ApiKey:            "upstream-secret",
			UpstreamModelName: "grok-4.5",
		},
	}
	adaptor := GetAdaptor(info.ApiType)
	require.NotNil(t, adaptor)
	adaptor.Init(info)

	upstreamPayload, newAPIError := normalizeResponsesWebsocketUpstreamPayload(c, adaptor, info, payload, request)

	require.Nil(t, newAPIError)
	assert.False(t, gjson.GetBytes(upstreamPayload, "stream").Exists())
	assert.False(t, gjson.GetBytes(upstreamPayload, "background").Exists())
	assert.Equal(t, "grok-4.5", gjson.GetBytes(upstreamPayload, "model").String())
}
