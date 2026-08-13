package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestCursorChannelSupportsOnlyChatCompletionsPath(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCursor}

	assert.True(t, channelSupportsRequestPath(channel, "/v1/chat/completions", "composer-2"))
	assert.True(t, channelSupportsRequestPath(channel, "/pg/chat/completions", "composer-2"))
	assert.False(t, channelSupportsRequestPath(channel, "/v1/responses", "composer-2"))
	assert.False(t, channelSupportsRequestPath(channel, "/v1/messages", "composer-2"))
}
