package newapi

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLPreservesClaudeCountTokensPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeClaudeCountTokens,
		RequestURLPath: "/v1/messages/count_tokens?beta=true",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://downstream.example",
			ChannelType:    constant.ChannelTypeNewAPI,
		},
	}

	got, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://downstream.example/v1/messages/count_tokens?beta=true", got)
}
