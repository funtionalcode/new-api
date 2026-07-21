package middleware

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func abortWithOpenAiMessage(c *gin.Context, statusCode int, message string, code ...types.ErrorCode) {
	codeStr := ""
	if len(code) > 0 {
		codeStr = string(code[0])
	}
	userId := c.GetInt("id")
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			"type":    "new_api_error",
			"code":    codeStr,
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), fmt.Sprintf("user %d | %s", userId, message))
	recordMiddlewareErrorLog(c, statusCode, message, codeStr)
}

func abortWithMidjourneyMessage(c *gin.Context, statusCode int, code int, description string) {
	c.JSON(statusCode, gin.H{
		"description": description,
		"type":        "new_api_error",
		"code":        code,
	})
	c.Abort()
	logger.LogError(c.Request.Context(), description)
}

func recordMiddlewareErrorLog(c *gin.Context, statusCode int, message string, errorCode string) {
	if !constant.ErrorLogEnabled || c == nil {
		return
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		return
	}
	tokenName := c.GetString("token_name")
	tokenId := c.GetInt("token_id")
	userGroup := c.GetString("group")
	modelName := c.GetString("original_model")
	if modelName == "" {
		modelName = common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	}
	channelId := c.GetInt("channel_id")
	if channelId == 0 {
		channelId = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	}
	other := map[string]interface{}{
		"error_type":  "new_api_error",
		"error_code":  errorCode,
		"status_code": statusCode,
	}
	if c.Request != nil && c.Request.URL != nil {
		other["request_path"] = c.Request.URL.Path
	}
	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	useTimeSeconds := 0
	if !startTime.IsZero() {
		useTimeSeconds = int(time.Since(startTime).Seconds())
	}
	content := message
	if statusCode > 0 {
		content = fmt.Sprintf("status_code=%d, %s", statusCode, message)
	}
	model.RecordErrorLog(
		c,
		userId,
		channelId,
		modelName,
		tokenName,
		content,
		tokenId,
		useTimeSeconds,
		false,
		userGroup,
		other,
	)
}
