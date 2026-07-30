package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/db_backup_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const dbBackupSchedulerTickInterval = 30 * time.Second

var (
	dbBackupSchedulerOnce    sync.Once
	dbBackupSchedulerRunning atomic.Bool
	// nextUnix is the next due fire time (unix seconds). 0 means "recompute".
	dbBackupNextFireUnix atomic.Int64
	// last cron expression + enabled flag used to recompute next fire on change.
	dbBackupScheduleFingerprint atomic.Value // string
)

// StartDBBackupScheduler runs a master-only ticker that enqueues host-side
// db_backup tasks according to the configured cron expression. It never claims
// or executes dumps in-process — the host agent remains the sole runner.
func StartDBBackupScheduler() {
	dbBackupSchedulerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		dbBackupScheduleFingerprint.Store("")
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"db backup scheduler started: tick=%s", dbBackupSchedulerTickInterval,
			))
			ticker := time.NewTicker(dbBackupSchedulerTickInterval)
			defer ticker.Stop()

			runDBBackupSchedulerOnce()
			for range ticker.C {
				runDBBackupSchedulerOnce()
			}
		})
	})
}

// ResetDBBackupSchedule recomputes the next fire time after config updates.
func ResetDBBackupSchedule() {
	dbBackupNextFireUnix.Store(0)
	dbBackupScheduleFingerprint.Store("")
}

func runDBBackupSchedulerOnce() {
	if !dbBackupSchedulerRunning.CompareAndSwap(false, true) {
		return
	}
	defer dbBackupSchedulerRunning.Store(false)

	cfg := db_backup_setting.GetDBBackupSetting()
	if !cfg.ScheduleEnabled {
		dbBackupNextFireUnix.Store(0)
		dbBackupScheduleFingerprint.Store("disabled")
		return
	}

	expr := strings.TrimSpace(cfg.CronExpression)
	if expr == "" {
		return
	}

	fingerprint := "enabled:" + expr
	prev, _ := dbBackupScheduleFingerprint.Load().(string)
	if prev != fingerprint {
		dbBackupScheduleFingerprint.Store(fingerprint)
		dbBackupNextFireUnix.Store(0)
	}

	sched, err := db_backup_setting.ParseCronSchedule(expr)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf(
			"db backup scheduler invalid cron %q: %v", expr, err,
		))
		return
	}

	now := time.Now()
	nextUnix := dbBackupNextFireUnix.Load()
	if nextUnix <= 0 {
		// First observation of this schedule: arm the *next* fire, do not backfill.
		next := sched.Next(now)
		if next.IsZero() {
			return
		}
		dbBackupNextFireUnix.Store(next.Unix())
		return
	}

	if now.Unix() < nextUnix {
		return
	}

	// Due: enqueue only. Host agent claims and executes.
	if _, _, err := TriggerDBBackup("scheduler"); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf(
			"db backup scheduler enqueue failed: %v", err,
		))
		// Still advance to avoid tight failure loops on every tick.
	}

	next := sched.Next(now)
	if next.IsZero() {
		dbBackupNextFireUnix.Store(0)
		return
	}
	// Guard against pathological schedules that return "now" repeatedly.
	if !next.After(now) {
		next = sched.Next(now.Add(time.Second))
	}
	dbBackupNextFireUnix.Store(next.Unix())
}
