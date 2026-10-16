package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetJevDailyUsage(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "permission denied"})
		return
	}
	start, startErr := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, endErr := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if startErr != nil || endErr != nil || start <= 0 || end <= start || end-start > 26*3600 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid daily time range"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	report, err := model.GetJevDailyUsage(ctx, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}
