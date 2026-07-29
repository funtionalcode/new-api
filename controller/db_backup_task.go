package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type reportDBBackupTaskRequest struct {
	TaskID      string                     `json:"task_id"`
	Success     bool                       `json:"success"`
	Artifacts   []service.DBBackupArtifact `json:"artifacts"`
	DurationMs  int64                      `json:"duration_ms"`
	Host        string                     `json:"host"`
	Error       string                     `json:"error"`
	TriggeredBy string                     `json:"triggered_by"`
}

func TriggerDBBackupTask(c *gin.Context) {
	username := c.GetString("username")
	task, created, err := service.TriggerDBBackup(username)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	message := ""
	if !created {
		message = "db backup task already active"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    task.ToResponse(),
	})
}

func GetPendingDBBackupTask(c *gin.Context) {
	task, err := service.ClaimPendingDBBackup()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if task == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    task.ToResponse(),
	})
}

func ReportDBBackupTask(c *gin.Context) {
	var req reportDBBackupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid request body",
		})
		return
	}

	payload := service.DBBackupPayload{
		TriggeredBy: req.TriggeredBy,
		TriggeredAt: common.GetTimestamp(),
	}
	result := service.DBBackupResult{
		Artifacts:  req.Artifacts,
		DurationMs: req.DurationMs,
		Host:       req.Host,
	}
	if err := service.FinishDBBackupReport(req.TaskID, req.Success, payload, result, req.Error); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
