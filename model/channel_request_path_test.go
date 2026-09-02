package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
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

	filter := func(path string) []int {
		channels, _ := filterCandidateIDs([]int{1, 2}, "claude-opus-5", []taskdto.ChannelFilter{{Kind: taskdto.FilterRequestPath, RequestPath: path}})
		return channels
	}
	assert.Equal(t, []int{1, 2}, filter("/v1/messages"))
	assert.Equal(t, []int{1, 2}, filter("/v1/responses"))
	assert.Equal(t, []int{2}, filter("/v1/responses/compact"))
}
