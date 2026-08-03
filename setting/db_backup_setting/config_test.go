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

func TestValidateAllowsDisablingClickHouseBackup(t *testing.T) {
	cfg := GetDBBackupSetting()
	cfg.CKContainer = ""
	cfg.CKUser = ""
	cfg.CKDatabases = ""

	require.NoError(t, Validate(cfg))
}

func TestValidateCronExpression(t *testing.T) {
	base := GetDBBackupSetting()

	ok := base
	ok.ScheduleEnabled = true
	ok.CronExpression = "0 3 * * 0"
	require.NoError(t, Validate(ok))

	daily := base
	daily.ScheduleEnabled = true
	daily.CronExpression = "30 2 * * *"
	require.NoError(t, Validate(daily))

	emptyRequired := base
	emptyRequired.ScheduleEnabled = true
	emptyRequired.CronExpression = ""
	assert.Error(t, Validate(emptyRequired))

	invalid := base
	invalid.ScheduleEnabled = true
	invalid.CronExpression = "not a cron"
	assert.Error(t, Validate(invalid))

	// Disabled schedule may omit expression.
	disabled := base
	disabled.ScheduleEnabled = false
	disabled.CronExpression = ""
	require.NoError(t, Validate(disabled))
}

func TestParseCronSchedule(t *testing.T) {
	sched, err := ParseCronSchedule("0 3 * * 0")
	require.NoError(t, err)
	require.NotNil(t, sched)

	_, err = ParseCronSchedule("")
	assert.Error(t, err)
}

func TestToEnvMap(t *testing.T) {
	env := ToEnvMap(GetDBBackupSetting())
	assert.Equal(t, "/data/backups/new-api", env["BACKUP_ROOT"])
	assert.Equal(t, "4", env["KEEP_WEEKLY"])
	_, hasToken := env["DB_BACKUP_AGENT_TOKEN"]
	assert.False(t, hasToken)
}

func TestToOptionMapIncludesSchedule(t *testing.T) {
	cfg := GetDBBackupSetting()
	cfg.ScheduleEnabled = true
	cfg.CronExpression = "15 4 * * 1"
	opts := ToOptionMap(cfg)
	assert.Equal(t, "true", opts["db_backup_setting.schedule_enabled"])
	assert.Equal(t, "15 4 * * 1", opts["db_backup_setting.cron_expression"])
}
