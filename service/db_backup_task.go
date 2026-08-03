package service

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/db_backup_setting"
)

//go:embed db_backup_script.default.sh
var defaultDBBackupScriptTemplate string

// DefaultDBBackupScriptTemplate returns the built-in host backup script template
// shown in the UI when no custom script has been saved yet.
func DefaultDBBackupScriptTemplate() string {
	return defaultDBBackupScriptTemplate
}

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
	LogPath    string             `json:"log_path,omitempty"`
	LogDir     string             `json:"log_dir,omitempty"`
	LogExcerpt string             `json:"log_excerpt,omitempty"`
}

type DBBackupConfigView struct {
	db_backup_setting.DBBackupSetting
	ScriptSHA256 string `json:"script_sha256"`
	HasScript    bool   `json:"has_script"`
}

type DBBackupScriptView struct {
	Content     string `json:"content"`
	SHA256      string `json:"sha256"`
	IsDefault   bool   `json:"is_default,omitempty"`
	DefaultHint string `json:"default_hint,omitempty"`
}

type DBBackupAgentBundle struct {
	Config          map[string]string `json:"config"`
	ScriptApply     bool              `json:"script_apply"`
	ScriptSHA256    string            `json:"script_sha256,omitempty"`
	ScriptContent   string            `json:"script_content,omitempty"`
	ScriptUnchanged bool              `json:"script_unchanged,omitempty"`
	ScriptPathHint  string            `json:"script_path_hint"`
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
	taskID = strings.TrimSpace(taskID)
	errMsg = strings.TrimSpace(errMsg)
	if taskID == "" {
		common.SysLog(fmt.Sprintf("ignore db backup report without task_id: host=%s triggered_by=%s", result.Host, payload.TriggeredBy))
		return nil
	}
	if !succeeded && errMsg == "" {
		errMsg = "backup failed: host agent reported failure without error detail"
	}

	status := model.SystemTaskStatusSucceeded
	if !succeeded {
		status = model.SystemTaskStatusFailed
	}

	if err := model.FinishSystemTask(taskID, dbBackupHostRunnerID, status, result, errMsg); err != nil {
		// Backup already finished on the host; do not fail the report just
		// because the lease expired or the row was already closed.
		common.SysLog(fmt.Sprintf("db backup finish task %s failed: %v", taskID, err))
	}

	model.RecordLog(0, model.LogTypeSystem, buildDBBackupLogContent(status, result, errMsg), "")
	return nil
}

func GetDBBackupConfigView() DBBackupConfigView {
	cfg := db_backup_setting.GetDBBackupSetting()
	content, sha := model.GetDBBackupScript()
	return DBBackupConfigView{
		DBBackupSetting: cfg,
		ScriptSHA256:    sha,
		HasScript:       content != "",
	}
}

func UpdateDBBackupConfig(cfg db_backup_setting.DBBackupSetting) error {
	if err := db_backup_setting.Validate(cfg); err != nil {
		return err
	}
	if err := model.UpdateOptionsBulk(db_backup_setting.ToOptionMap(cfg)); err != nil {
		return err
	}
	// Option map update reloads settings asynchronously via the option watcher;
	// reset the in-process next-fire so the scheduler re-arms from the new cron.
	ResetDBBackupSchedule()
	return nil
}

func GetDBBackupScriptView() DBBackupScriptView {
	content, sha := model.GetDBBackupScript()
	if strings.TrimSpace(content) == "" {
		return DBBackupScriptView{
			Content:     DefaultDBBackupScriptTemplate(),
			SHA256:      "",
			IsDefault:   true,
			DefaultHint: "Showing the default template. Saving will materialize it on the host agent.",
		}
	}
	return DBBackupScriptView{Content: content, SHA256: sha}
}

func UpdateDBBackupScript(content string, confirm bool) (string, error) {
	if !confirm {
		return "", fmt.Errorf("confirm must be true to update backup script")
	}
	return model.SetDBBackupScript(content)
}

func BuildDBBackupAgentBundle(localScriptSHA string) DBBackupAgentBundle {
	cfg := db_backup_setting.GetDBBackupSetting()
	content, sha := model.GetDBBackupScript()
	bundle := DBBackupAgentBundle{
		Config:         db_backup_setting.ToEnvMap(cfg),
		ScriptPathHint: "/usr/local/bin/backup-new-api-db.sh",
	}

	if !cfg.ScriptEnabled || content == "" {
		bundle.ScriptApply = false
		return bundle
	}

	bundle.ScriptApply = true
	bundle.ScriptSHA256 = sha
	if localScriptSHA != "" && localScriptSHA == sha {
		bundle.ScriptUnchanged = true
		return bundle
	}
	bundle.ScriptContent = content
	return bundle
}

const maxDBBackupLogExcerptRunes = 32000

// TruncateDBBackupLogExcerpt bounds agent-reported log text stored on the task.
func TruncateDBBackupLogExcerpt(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxDBBackupLogExcerptRunes {
		return s
	}
	return string(runes[len(runes)-maxDBBackupLogExcerptRunes:])
}

func buildDBBackupLogContent(status model.SystemTaskStatus, result DBBackupResult, errMsg string) string {
	parts := []string{fmt.Sprintf("db_backup %s", status)}
	if result.Host != "" {
		parts = append(parts, "host="+result.Host)
	}
	if result.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("duration_ms=%d", result.DurationMs))
	}
	if result.LogPath != "" {
		parts = append(parts, "log="+result.LogPath)
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
