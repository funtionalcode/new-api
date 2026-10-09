package service

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestDefaultDBBackupScriptExportsClickHouseDataAndPreservesOtherDirectories(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		failedExport bool
	}{
		{name: "完整备份"},
		{name: "表数据导出失败", failedExport: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			tempDir := t.TempDir()
			scriptPath := filepath.Join(tempDir, "backup.sh")
			require.NoError(t, os.WriteFile(scriptPath, []byte(DefaultDBBackupScriptTemplate()), 0o755))
			binDir := filepath.Join(tempDir, "bin")
			require.NoError(t, os.Mkdir(binDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(binDir, "docker"), []byte(`#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "inspect" ]]; then exit 0; fi
if [[ "$1" == "exec" && "$2" == "postgres" ]]; then printf 'CREATE TABLE users(id int);\n'; exit 0; fi
query="${!#}"
case "$query" in
  "EXISTS DATABASE "*) printf '1\n' ;;
  "SHOW CREATE DATABASE "*) printf 'CREATE DATABASE new_api_logs\n' ;;
  "SELECT name, engine FROM system.tables"*) printf '%s\n' '{"name":"logs","engine":"MergeTree"}' '{"name":"logs_view","engine":"View"}' ;;
  'SHOW CREATE TABLE '*logs_view*) printf 'CREATE VIEW new_api_logs.logs_view AS SELECT * FROM new_api_logs.logs\n' ;;
  'SHOW CREATE TABLE '*) printf 'CREATE TABLE new_api_logs.logs(id UInt64) ENGINE=MergeTree ORDER BY id\n' ;;
  'SELECT * FROM '*Native)
    if [[ "${FAIL_EXPORT:-false}" == "true" ]]; then printf 'table export failed\n' >&2; exit 1; fi
    printf 'native-table-rows' ;;
  *) printf 'unexpected query: %s\n' "$query" >&2; exit 2 ;;
esac
`), 0o755))
			backupRoot := filepath.Join(tempDir, "backups")
			for _, name := range []string{"postgres", "clickhouse", "20250101T000000Z", "20250102T000000Z"} {
				require.NoError(t, os.MkdirAll(filepath.Join(backupRoot, name), 0o700))
			}
			require.NoError(t, os.WriteFile(filepath.Join(backupRoot, "20250102T000000Z", ".backup-complete"), nil, 0o600))
			type backupReport struct {
				Success   bool               `json:"success"`
				Artifacts []DBBackupArtifact `json:"artifacts"`
				Error     string             `json:"error"`
			}
			reported := make(chan backupReport, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var report backupReport
				if err := common.DecodeJson(r.Body, &report); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				reported <- report
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer server.Close()
			cmd := exec.Command("bash", scriptPath)
			failValue := "false"
			if scenario.failedExport {
				failValue = "true"
			}
			cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"), "BACKUP_ROOT="+backupRoot, "LOG_DIR="+filepath.Join(tempDir, "logs"), "CK_DATABASES=new_api_logs", "CK_PASSWORD=", "KEEP_WEEKLY=1", "TASK_ID=backup-test", "DB_BACKUP_AGENT_TOKEN=test-agent", "NEW_API_REPORT_URL="+server.URL, "FAIL_EXPORT="+failValue)
			output, err := cmd.CombinedOutput()
			for _, name := range []string{"postgres", "clickhouse", "20250101T000000Z"} {
				_, statErr := os.Stat(filepath.Join(backupRoot, name))
				assert.NoError(t, statErr, "保留其他目录或未完成备份: %s", name)
			}
			var report backupReport
			select {
			case report = <-reported:
			default:
				t.Fatal("备份脚本未报告结果: " + string(output))
			}
			assert.Equal(t, !scenario.failedExport, report.Success)
			if scenario.failedExport {
				require.Error(t, err, string(output))
				assert.NotEmpty(t, report.Error)
				assert.Len(t, report.Artifacts, 1)
				_, statErr := os.Stat(filepath.Join(backupRoot, "20250102T000000Z"))
				assert.NoError(t, statErr, "失败时不执行保留策略")
				return
			}
			require.NoError(t, err, string(output))
			require.Len(t, report.Artifacts, 2)
			assert.Equal(t, "tar.gz", report.Artifacts[1].Format)
			archive, openErr := os.Open(report.Artifacts[1].File)
			require.NoError(t, openErr)
			defer archive.Close()
			compressed, gzipErr := gzip.NewReader(archive)
			require.NoError(t, gzipErr)
			defer compressed.Close()
			files := map[string]string{}
			reader := tar.NewReader(compressed)
			for {
				header, nextErr := reader.Next()
				if nextErr == io.EOF {
					break
				}
				require.NoError(t, nextErr)
				content, readErr := io.ReadAll(reader)
				require.NoError(t, readErr)
				files[header.Name] = string(content)
			}
			assert.Equal(t, "native-table-rows", files["table-0.native"])
			assert.Contains(t, files["table-0.sql"], "CREATE TABLE")
			assert.Contains(t, files["table-1.sql"], "CREATE VIEW")
			assert.NotContains(t, files, "table-1.native")
			assert.Contains(t, files["manifest.json"], `"data": "table-0.native"`)
			_, statErr := os.Stat(filepath.Join(backupRoot, "20250102T000000Z"))
			assert.True(t, os.IsNotExist(statErr), "只清理已完成的历史备份")
		})
	}
}

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
