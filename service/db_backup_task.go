package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const dbBackupHostRunnerID = "host-backup-agent"

type DBBackupPayload struct {
	TriggeredBy string `json:"triggered_by,omitempty"`
	TriggeredAt int64  `json:"triggered_at,omitempty"`
}

type DBBackupArtifact struct {
	Type      string `json:"type"`
	Database  string `json:"database,omitempty"`
	Container string `json:"container,omitempty"`
	File      string `json:"file"`
	SizeBytes int64  `json:"size_bytes"`
	Sha256    string `json:"sha256"`
	Format    string `json:"format,omitempty"`
}

type DBBackupResult struct {
	Artifacts  []DBBackupArtifact `json:"artifacts"`
	DurationMs int64              `json:"duration_ms,omitempty"`
	Host       string             `json:"host,omitempty"`
}

func TriggerDBBackup(triggeredBy string) (*model.SystemTask, bool, error) {
	payload := DBBackupPayload{
		TriggeredBy: triggeredBy,
		TriggeredAt: common.GetTimestamp(),
	}
	return EnqueueSystemTask(model.SystemTaskTypeDBBackup, payload)
}

func ClaimPendingDBBackup() (*model.SystemTask, error) {
	tasks, err := model.FindPendingSystemTasks(model.SystemTaskTypeDBBackup, 1)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}

	ttlMinutes := common.GetEnvOrDefault("DB_BACKUP_LOCK_TTL_MINUTES", 30)
	if ttlMinutes <= 0 {
		ttlMinutes = 30
	}
	lockUntil := common.GetTimestamp() + int64(ttlMinutes*60)
	claimed, ok, err := model.ClaimSystemTask(tasks[0].ID, model.SystemTaskTypeDBBackup, dbBackupHostRunnerID, lockUntil)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return claimed, nil
}

func FinishDBBackupReport(taskID string, succeeded bool, payload DBBackupPayload, result DBBackupResult, errMsg string) error {
	status := model.SystemTaskStatusSucceeded
	if !succeeded {
		status = model.SystemTaskStatusFailed
	}

	if taskID != "" {
		if err := model.FinishSystemTask(taskID, dbBackupHostRunnerID, status, result, errMsg); err != nil {
			// Backup already finished on the host; do not fail the report just
			// because the lease expired or the row was already closed.
			common.SysLog(fmt.Sprintf("db backup finish task %s failed: %v", taskID, err))
		}
	} else {
		if _, err := model.CreateCompletedSystemTask(model.SystemTaskTypeDBBackup, payload, status, result, errMsg, dbBackupHostRunnerID); err != nil {
			return err
		}
	}

	model.RecordLog(0, model.LogTypeSystem, buildDBBackupLogContent(status, result, errMsg), "")
	return nil
}

func buildDBBackupLogContent(status model.SystemTaskStatus, result DBBackupResult, errMsg string) string {
	parts := []string{fmt.Sprintf("db_backup %s", status)}
	if result.Host != "" {
		parts = append(parts, "host="+result.Host)
	}
	if result.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("duration_ms=%d", result.DurationMs))
	}
	if len(result.Artifacts) > 0 {
		files := make([]string, 0, len(result.Artifacts))
		for _, artifact := range result.Artifacts {
			label := artifact.Type
			if artifact.File != "" {
				label = fmt.Sprintf("%s:%s", artifact.Type, artifact.File)
			}
			if artifact.SizeBytes > 0 {
				label = fmt.Sprintf("%s(%dB)", label, artifact.SizeBytes)
			}
			files = append(files, label)
		}
		parts = append(parts, "artifacts="+strings.Join(files, ","))
	}
	if errMsg != "" {
		parts = append(parts, "error="+errMsg)
	}
	return strings.Join(parts, " ")
}
