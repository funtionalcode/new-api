package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/db_backup_setting"

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
	LogPath     string                     `json:"log_path"`
	LogDir      string                     `json:"log_dir"`
	LogExcerpt  string                     `json:"log_excerpt"`
}

type updateDBBackupScriptRequest struct {
	Content string `json:"content"`
	Confirm bool   `json:"confirm"`
}

type agentBundleRequest struct {
	LocalScriptSHA256 string `json:"local_script_sha256"`
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

func GetDBBackupConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetDBBackupConfigView(),
	})
}

func UpdateDBBackupConfig(c *gin.Context) {
	var req db_backup_setting.DBBackupSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid request body",
		})
		return
	}
	if err := service.UpdateDBBackupConfig(req); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetDBBackupConfigView(),
	})
}

func GetDBBackupScript(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetDBBackupScriptView(),
	})
}

func UpdateDBBackupScript(c *gin.Context) {
	var req updateDBBackupScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid request body",
		})
		return
	}
	sha, err := service.UpdateDBBackupScript(req.Content, req.Confirm)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"sha256": sha,
		},
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

func GetDBBackupAgentBundle(c *gin.Context) {
	var req agentBundleRequest
	_ = c.ShouldBindJSON(&req)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.BuildDBBackupAgentBundle(req.LocalScriptSHA256),
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
		LogPath:    req.LogPath,
		LogDir:     req.LogDir,
		LogExcerpt: service.TruncateDBBackupLogExcerpt(req.LogExcerpt),
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
