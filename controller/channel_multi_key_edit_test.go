package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateChannelConvertsSingleKeyChannelToMultiKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:channel-multi-key-edit?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
	})

	channel := model.Channel{
		Type:        constant.ChannelTypeCursor,
		Key:         "cursor-key-one",
		Status:      common.ChannelStatusEnabled,
		Name:        "Cursor pool",
		Models:      "gpt-5.6",
		Group:       "default",
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&channel).Error)

	body, err := common.Marshal(map[string]any{
		"id":             channel.Id,
		"type":           channel.Type,
		"key":            "cursor-key-two\ncursor-key-three",
		"name":           channel.Name,
		"models":         channel.Models,
		"group":          channel.Group,
		"multi_key_mode": "polling",
		"key_mode":       "append",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)

	UpdateChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	assert.Equal(t, "cursor-key-one\ncursor-key-two\ncursor-key-three", saved.Key)
	assert.True(t, saved.ChannelInfo.IsMultiKey)
	assert.Equal(t, 3, saved.ChannelInfo.MultiKeySize)
	assert.Equal(t, constant.MultiKeyModePolling, saved.ChannelInfo.MultiKeyMode)
}
