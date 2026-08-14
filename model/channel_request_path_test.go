package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestCursorChannelSupportsTextRequestPaths(t *testing.T) {
	channel := &Channel{Type: constant.ChannelTypeCursor}

	assert.True(t, channel.SupportsRequestPath("/v1/chat/completions", "claude-opus-5"))
	assert.True(t, channel.SupportsRequestPath("/pg/chat/completions", "claude-opus-5"))
	assert.True(t, channel.SupportsRequestPath("/v1/messages", "claude-opus-5"))
	assert.False(t, channel.SupportsRequestPath("/v1/messages/count_tokens", "claude-opus-5"))
	assert.True(t, channel.SupportsRequestPath("/v1/responses", "claude-opus-5"))
	assert.False(t, channel.SupportsRequestPath("/v1/responses/compact", "claude-opus-5"))
}

func TestCursorChannelIsFilteredFromUnsupportedCachedRequestPaths(t *testing.T) {
	originalChannels := channelsIDM
	originalConfigs := channel2advancedCustomConfig
	t.Cleanup(func() {
		channelsIDM = originalChannels
		channel2advancedCustomConfig = originalConfigs
	})
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Type: constant.ChannelTypeCursor},
		2: {Id: 2, Type: constant.ChannelTypeOpenAI},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{}

	assert.Equal(t, []int{1, 2}, filterChannelsByRequestPathAndModel([]int{1, 2}, "/v1/messages", "claude-opus-5"))
	assert.Equal(t, []int{1, 2}, filterChannelsByRequestPathAndModel([]int{1, 2}, "/v1/responses", "claude-opus-5"))
	assert.Equal(t, []int{2}, filterChannelsByRequestPathAndModel([]int{1, 2}, "/v1/responses/compact", "claude-opus-5"))
}
