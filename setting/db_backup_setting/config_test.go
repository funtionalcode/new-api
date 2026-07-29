package db_backup_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDefaults(t *testing.T) {
	require.NoError(t, Validate(GetDBBackupSetting()))
}

func TestValidateRejects(t *testing.T) {
	base := GetDBBackupSetting()

	badPath := base
	badPath.BackupRoot = "relative/path"
	assert.Error(t, Validate(badPath))

	dotDot := base
	dotDot.LogDir = "/var/log/../etc"
	assert.Error(t, Validate(dotDot))

	keep := base
	keep.KeepWeekly = 0
	assert.Error(t, Validate(keep))
	keep.KeepWeekly = 100
	assert.Error(t, Validate(keep))

	name := base
	name.PGContainer = "bad name"
	assert.Error(t, Validate(name))

	dbs := base
	dbs.CKDatabases = "ok;rm -rf"
	assert.Error(t, Validate(dbs))
}

func TestToEnvMap(t *testing.T) {
	env := ToEnvMap(GetDBBackupSetting())
	assert.Equal(t, "/data/backups/new-api", env["BACKUP_ROOT"])
	assert.Equal(t, "4", env["KEEP_WEEKLY"])
	_, hasToken := env["DB_BACKUP_AGENT_TOKEN"]
	assert.False(t, hasToken)
}
