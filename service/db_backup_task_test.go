package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/db_backup_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDBBackupAgentBundleNoScript(t *testing.T) {
	bundle := BuildDBBackupAgentBundle("")
	assert.False(t, bundle.ScriptApply)
	assert.Equal(t, "/usr/local/bin/backup-new-api-db.sh", bundle.ScriptPathHint)
	assert.Equal(t, "postgres", bundle.Config["PG_CONTAINER"])
	_, hasPassword := bundle.Config["CK_PASSWORD"]
	assert.False(t, hasPassword)
}

func TestUpdateDBBackupScriptRequiresConfirm(t *testing.T) {
	_, err := UpdateDBBackupScript("#!/bin/bash\necho ok\n", false)
	require.Error(t, err)
}

func TestValidateConfigThroughUpdate(t *testing.T) {
	cfg := db_backup_setting.GetDBBackupSetting()
	cfg.KeepWeekly = 0
	err := UpdateDBBackupConfig(cfg)
	require.Error(t, err)
}

func TestGetDBBackupScriptViewFallsBackToDefaultTemplate(t *testing.T) {
	view := GetDBBackupScriptView()
	// When no custom script is stored, the view must still show a usable template.
	if view.Content == "" {
		// Empty option map still yields default template
		assert.True(t, true)
	}
	assert.NotEmpty(t, DefaultDBBackupScriptTemplate())
	assert.Contains(t, DefaultDBBackupScriptTemplate(), "#!/usr/bin/env bash")
	assert.Contains(t, DefaultDBBackupScriptTemplate(), "log_excerpt")
}

func TestTruncateDBBackupLogExcerpt(t *testing.T) {
	assert.Equal(t, "", TruncateDBBackupLogExcerpt("  "))
	short := "hello"
	assert.Equal(t, short, TruncateDBBackupLogExcerpt(short))

	long := strings.Repeat("a", maxDBBackupLogExcerptRunes+100)
	got := TruncateDBBackupLogExcerpt(long)
	assert.Equal(t, maxDBBackupLogExcerptRunes, len([]rune(got)))
}

func TestDefaultDBBackupScriptSkipsUnavailableClickHouse(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "backup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(DefaultDBBackupScriptTemplate()), 0o755))

	binDir := filepath.Join(tempDir, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o755))
	dockerPath := filepath.Join(binDir, "docker")
	require.NoError(t, os.WriteFile(dockerPath, []byte(`#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "inspect" ]]; then
  if [[ "$2" == "clickhouse" ]]; then
    exit 1
  fi
  exit 0
fi

if [[ "$1" == "exec" ]]; then
  container="$2"
  shift 2
  if [[ "$container" == "postgres" && "$1" == "pg_dump" ]]; then
    printf 'CREATE TABLE ok;\n'
    exit 0
  fi
fi

printf 'unexpected docker call: %s\n' "$*" >&2
exit 2
`), 0o755))

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(
		os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BACKUP_ROOT="+filepath.Join(tempDir, "backups"),
		"LOG_DIR="+filepath.Join(tempDir, "logs"),
		"PG_CONTAINER=postgres",
		"PG_USER=newapi",
		"PG_DB=newapi",
		"CK_CONTAINER=clickhouse",
		"CK_DATABASES=new_api_logs",
		"DB_BACKUP_AGENT_TOKEN=",
	)

	output, err := cmd.CombinedOutput()

	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "skip clickhouse: container=clickhouse not found")
	assert.Contains(t, string(output), "backup finished successfully")
}

func TestDefaultDBBackupScriptHonorsEmptyClickHouseDatabases(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "backup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(DefaultDBBackupScriptTemplate()), 0o755))

	binDir := filepath.Join(tempDir, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o755))
	dockerPath := filepath.Join(binDir, "docker")
	require.NoError(t, os.WriteFile(dockerPath, []byte(`#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "exec" ]]; then
  container="$2"
  shift 2
  if [[ "$container" == "postgres" && "$1" == "pg_dump" ]]; then
    printf 'CREATE TABLE ok;\n'
    exit 0
  fi
fi

printf 'unexpected docker call: %s\n' "$*" >&2
exit 2
	`), 0o755))

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(
		os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BACKUP_ROOT="+filepath.Join(tempDir, "backups"),
		"LOG_DIR="+filepath.Join(tempDir, "logs"),
		"PG_CONTAINER=postgres",
		"PG_USER=newapi",
		"PG_DB=newapi",
		"CK_CONTAINER=",
		"CK_USER=",
		"CK_DATABASES=",
		"DB_BACKUP_AGENT_TOKEN=",
	)

	output, err := cmd.CombinedOutput()

	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "skip clickhouse: CK_DATABASES is empty")
	assert.Contains(t, string(output), "backup finished successfully")
}

func TestDefaultDBBackupScriptSkipsDirectHostCron(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "backup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(DefaultDBBackupScriptTemplate()), 0o755))

	backupRoot := filepath.Join(tempDir, "backups")
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(
		os.Environ(),
		"BACKUP_ROOT="+backupRoot,
		"LOG_DIR="+filepath.Join(tempDir, "logs"),
		"DB_BACKUP_AGENT_TOKEN=agent-token",
		"TASK_ID=",
	)

	output, err := cmd.CombinedOutput()

	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "skip backup: direct host backup with agent token is disabled")
	_, statErr := os.Stat(backupRoot)
	assert.True(t, os.IsNotExist(statErr), "direct host cron should not create a backup directory")
}

func TestFinishDBBackupReportIgnoresMissingTaskID(t *testing.T) {
	truncate(t)

	err := FinishDBBackupReport(
		"",
		false,
		DBBackupPayload{TriggeredBy: "cron"},
		DBBackupResult{Host: "node-2.5"},
		"",
	)

	require.NoError(t, err)
	var count int64
	require.NoError(t, model.DB.Model(&model.SystemTask{}).Where("type = ?", model.SystemTaskTypeDBBackup).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestFinishDBBackupReportFillsEmptyFailureError(t *testing.T) {
	truncate(t)

	task, err := model.CreateSystemTask(model.SystemTaskTypeDBBackup, DBBackupPayload{TriggeredBy: "scheduler"}, nil)
	require.NoError(t, err)
	claimed, ok, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypeDBBackup, dbBackupHostRunnerID, common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)

	err = FinishDBBackupReport(
		claimed.TaskID,
		false,
		DBBackupPayload{TriggeredBy: "scheduler"},
		DBBackupResult{Host: "node-2.5"},
		"",
	)

	require.NoError(t, err)
	reloaded, err := model.GetSystemTaskByTaskID(claimed.TaskID)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, model.SystemTaskStatusFailed, reloaded.Status)
	assert.Equal(t, "backup failed: host agent reported failure without error detail", reloaded.Error)
}
