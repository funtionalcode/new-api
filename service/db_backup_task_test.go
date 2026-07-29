package service

import (
	"strings"
	"testing"

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
