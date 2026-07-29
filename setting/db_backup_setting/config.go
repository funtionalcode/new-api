package db_backup_setting

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/setting/config"
)

// DBBackupSetting holds non-secret host backup parameters managed from the UI.
// Secrets (CK password, agent token) stay on the host only.
type DBBackupSetting struct {
	BackupRoot   string `json:"backup_root"`
	PGContainer  string `json:"pg_container"`
	CKContainer  string `json:"ck_container"`
	PGUser       string `json:"pg_user"`
	PGDB         string `json:"pg_db"`
	CKUser       string `json:"ck_user"`
	CKDatabases  string `json:"ck_databases"`
	KeepWeekly   int    `json:"keep_weekly"`
	LogDir       string `json:"log_dir"`
	ScriptEnabled bool  `json:"script_enabled"`
}

var dbBackupSetting = DBBackupSetting{
	BackupRoot:    "/data/backups/new-api",
	PGContainer:   "postgres",
	CKContainer:   "clickhouse",
	PGUser:        "newapi",
	PGDB:          "newapi",
	CKUser:        "default",
	CKDatabases:   "new_api_logs clash_metrics",
	KeepWeekly:    4,
	LogDir:        "/var/log/new-api-backup",
	ScriptEnabled: true,
}

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	dbListPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?:[ \t]+[A-Za-z0-9][A-Za-z0-9._-]*)*$`)
)

func init() {
	config.GlobalConfig.Register("db_backup_setting", &dbBackupSetting)
}

func GetDBBackupSetting() DBBackupSetting {
	return dbBackupSetting
}

func Snapshot() DBBackupSetting {
	return dbBackupSetting
}

// Validate checks UI-managed backup parameters.
func Validate(s DBBackupSetting) error {
	if err := validateAbsPath(s.BackupRoot, "backup_root"); err != nil {
		return err
	}
	if err := validateAbsPath(s.LogDir, "log_dir"); err != nil {
		return err
	}
	if err := validateName(s.PGContainer, "pg_container"); err != nil {
		return err
	}
	if err := validateName(s.CKContainer, "ck_container"); err != nil {
		return err
	}
	if err := validateName(s.PGUser, "pg_user"); err != nil {
		return err
	}
	if err := validateName(s.PGDB, "pg_db"); err != nil {
		return err
	}
	if err := validateName(s.CKUser, "ck_user"); err != nil {
		return err
	}
	ckDatabases := strings.TrimSpace(s.CKDatabases)
	if ckDatabases == "" || !dbListPattern.MatchString(ckDatabases) {
		return fmt.Errorf("invalid ck_databases")
	}
	if s.KeepWeekly < 1 || s.KeepWeekly > 52 {
		return fmt.Errorf("keep_weekly must be between 1 and 52")
	}
	return nil
}

func validateName(value, field string) error {
	value = strings.TrimSpace(value)
	if value == "" || !namePattern.MatchString(value) {
		return fmt.Errorf("invalid %s", field)
	}
	return nil
}

func validateAbsPath(value, field string) error {
	value = strings.TrimSpace(value)
	if value == "" || !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be an absolute path", field)
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("%s must not contain '..'", field)
	}
	if strings.ContainsAny(value, "\x00\n\r") {
		return fmt.Errorf("invalid %s", field)
	}
	if utf8.RuneCountInString(value) > 512 {
		return fmt.Errorf("%s is too long", field)
	}
	return nil
}

// ToOptionMap returns flattened option keys for bulk update.
func ToOptionMap(s DBBackupSetting) map[string]string {
	return map[string]string{
		"db_backup_setting.backup_root":    strings.TrimSpace(s.BackupRoot),
		"db_backup_setting.pg_container":   strings.TrimSpace(s.PGContainer),
		"db_backup_setting.ck_container":   strings.TrimSpace(s.CKContainer),
		"db_backup_setting.pg_user":        strings.TrimSpace(s.PGUser),
		"db_backup_setting.pg_db":          strings.TrimSpace(s.PGDB),
		"db_backup_setting.ck_user":        strings.TrimSpace(s.CKUser),
		"db_backup_setting.ck_databases":   strings.TrimSpace(s.CKDatabases),
		"db_backup_setting.keep_weekly":    fmt.Sprintf("%d", s.KeepWeekly),
		"db_backup_setting.log_dir":        strings.TrimSpace(s.LogDir),
		"db_backup_setting.script_enabled": fmt.Sprintf("%t", s.ScriptEnabled),
	}
}

// ToEnvMap returns host env assignments for non-secret parameters.
func ToEnvMap(s DBBackupSetting) map[string]string {
	return map[string]string{
		"BACKUP_ROOT":   strings.TrimSpace(s.BackupRoot),
		"PG_CONTAINER":  strings.TrimSpace(s.PGContainer),
		"CK_CONTAINER":  strings.TrimSpace(s.CKContainer),
		"PG_USER":       strings.TrimSpace(s.PGUser),
		"PG_DB":         strings.TrimSpace(s.PGDB),
		"CK_USER":       strings.TrimSpace(s.CKUser),
		"CK_DATABASES":  strings.TrimSpace(s.CKDatabases),
		"KEEP_WEEKLY":   fmt.Sprintf("%d", s.KeepWeekly),
		"LOG_DIR":       strings.TrimSpace(s.LogDir),
	}
}
