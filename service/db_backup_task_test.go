package service

import (
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
