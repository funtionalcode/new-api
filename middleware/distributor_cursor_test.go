package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorChannelSupportsTextChatPaths(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeCursor}

	assert.True(t, channelSupportsRequestPath(channel, "/v1/chat/completions", "composer-2"))
	assert.True(t, channelSupportsRequestPath(channel, "/pg/chat/completions", "composer-2"))
	assert.True(t, channelSupportsRequestPath(channel, "/v1/messages", "composer-2"))
	assert.False(t, channelSupportsRequestPath(channel, "/v1/responses", "composer-2"))
}

func TestSetupContextForSelectedCursorChannelUsesSavedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelKeyIndexOverride, 1)
	channel := &model.Channel{
		Id:          42,
		Type:        constant.ChannelTypeCursor,
		Key:         "first-key\nsecond-key",
		Models:      "composer-2",
		Group:       "default",
		Status:      common.ChannelStatusEnabled,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true},
	}

	apiErr := SetupContextForSelectedChannel(ctx, channel, "composer-2")

	require.Nil(t, apiErr)
	assert.Equal(t, "second-key", common.GetContextKeyString(ctx, constant.ContextKeyChannelKey))
	assert.Equal(t, 1, common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex))
}

func TestSetupContextForSelectedCursorChannelRejectsUnavailableSavedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyChannelKeyIndexOverride, 1)
	channel := &model.Channel{
		Id:          42,
		Type:        constant.ChannelTypeCursor,
		Key:         "only-key",
		Models:      "composer-2",
		Group:       "default",
		Status:      common.ChannelStatusEnabled,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true},
	}

	apiErr := SetupContextForSelectedChannel(ctx, channel, "composer-2")

	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "saved key index is unavailable")
}
