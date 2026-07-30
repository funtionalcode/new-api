package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/db_backup_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDBBackupSchedulerArmsNextWithoutFiring(t *testing.T) {
	ResetDBBackupSchedule()
	// Force disabled first so we do not enqueue against a real DB.
	// The scheduler reads live GetDBBackupSetting(); we only assert re-arm logic
	// for disabled → no next fire.
	cfg := db_backup_setting.GetDBBackupSetting()
	require.False(t, cfg.ScheduleEnabled || false && cfg.ScheduleEnabled)

	// With default ScheduleEnabled=false the scheduler should clear next fire.
	runDBBackupSchedulerOnce()
	assert.Equal(t, int64(0), dbBackupNextFireUnix.Load())
}

func TestResetDBBackupScheduleClearsState(t *testing.T) {
	dbBackupNextFireUnix.Store(time.Now().Add(time.Hour).Unix())
	dbBackupScheduleFingerprint.Store("enabled:0 3 * * 0")
	ResetDBBackupSchedule()
	assert.Equal(t, int64(0), dbBackupNextFireUnix.Load())
	fp, _ := dbBackupScheduleFingerprint.Load().(string)
	assert.Equal(t, "", fp)
}
