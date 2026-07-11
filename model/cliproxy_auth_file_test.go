package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyCliproxyAuthFileBinding struct {
	Id          int    `gorm:"primaryKey"`
	UserId      int    `gorm:"index;not null"`
	Username    string `gorm:"size:64;index;default:''"`
	AuthIndex   string `gorm:"size:128;uniqueIndex;not null"`
	AuthName    string `gorm:"size:255;default:''"`
	AuthFile    string `gorm:"type:text"`
	Description string `gorm:"type:text"`
	AccountId   string `gorm:"size:128;index;default:''"`
	Enabled     bool   `gorm:"default:true"`
	CreatedAt   int64  `gorm:"bigint;index"`
	UpdatedAt   int64  `gorm:"bigint"`
}

func (legacyCliproxyAuthFileBinding) TableName() string {
	return "cliproxy_auth_file_bindings"
}

func TestGetCliproxyAuthFileBindingsSortsByPlanRank(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &CliproxyAuthFileBinding{}))
	DB = db

	for _, binding := range []*CliproxyAuthFileBinding{
		{UserId: 1, Username: "u", AuthIndex: "free", AuthName: "free.json", LastPlanType: "free", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "plus", AuthName: "plus.json", LastPlanType: "plus", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "prolite", AuthName: "prolite.json", LastPlanType: "prolite", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "pro", AuthName: "pro.json", LastPlanType: "pro", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "claude-max", AuthName: "claude-max.json", LastPlanType: "plan_max", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "claude-pro", AuthName: "claude-pro.json", LastPlanType: "plan_pro", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "team", AuthName: "team.json", LastPlanType: "team", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "unknown", AuthName: "unknown.json", LastPlanType: "enterprise", Enabled: true},
		{UserId: 1, Username: "u", AuthIndex: "empty", AuthName: "empty.json", Enabled: true},
	} {
		require.NoError(t, db.Create(binding).Error)
	}

	bindings, total, err := GetCliproxyAuthFileBindings(CliproxyAuthFileBindingQuery{}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	names := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		names = append(names, binding.AuthName)
	}

	require.Equal(t, []string{
		"claude-max.json",
		"pro.json",
		"prolite.json",
		"team.json",
		"claude-pro.json",
		"plus.json",
		"free.json",
		"unknown.json",
		"empty.json",
	}, names)
}

func TestGetCliproxyAuthFileBindingsIncludesUserRemark(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &CliproxyAuthFileBinding{}))
	DB = db

	require.NoError(t, db.Create(&User{
		Id:       7,
		Username: "demo",
		Remark:   "重点用户",
	}).Error)
	require.NoError(t, db.Create(&CliproxyAuthFileBinding{
		UserId:    7,
		Username:  "demo",
		AuthIndex: "auth-demo",
		AuthName:  "auth-demo.json",
		Enabled:   true,
	}).Error)

	bindings, total, err := GetCliproxyAuthFileBindings(CliproxyAuthFileBindingQuery{}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "重点用户", bindings[0].Remark)
}

func TestCliproxyAuthFileBindingUsesStableXAIProductUsageColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))

	require.True(t, db.Migrator().HasColumn(&CliproxyAuthFileBinding{}, "last_xai_product_usage"))
	require.False(t, db.Migrator().HasColumn(&CliproxyAuthFileBinding{}, "last_xa_iproduct_usage"))
}

func TestMigrateCliproxyAuthFileBindingNoteRenamesLegacyDescription(t *testing.T) {
	originalDB := DB
	originalMainDatabaseType := common.MainDatabaseType()
	t.Cleanup(func() {
		DB = originalDB
		common.SetMainDatabaseType(originalMainDatabaseType)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)

	require.NoError(t, db.AutoMigrate(&legacyCliproxyAuthFileBinding{}))
	require.NoError(t, db.Create(&legacyCliproxyAuthFileBinding{
		Id:          1,
		UserId:      7,
		Username:    "demo",
		AuthIndex:   "auth-demo",
		AuthName:    "auth-demo.json",
		Description: "旧备注",
		Enabled:     true,
	}).Error)

	require.NoError(t, migrateCliproxyAuthFileBindingNote())
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))

	hasNote, err := hasColumnByName("cliproxy_auth_file_bindings", "note")
	require.NoError(t, err)
	require.True(t, hasNote)
	hasDescription, err := hasColumnByName("cliproxy_auth_file_bindings", "description")
	require.NoError(t, err)
	require.False(t, hasDescription)

	binding, err := GetCliproxyAuthFileBindingById(1)
	require.NoError(t, err)
	require.Equal(t, "旧备注", binding.Note)
}

func TestUpdateCliproxyAuthFileBindingUsagePreservesLastUsageOnError(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))
	DB = db

	require.NoError(t, db.Create(&CliproxyAuthFileBinding{
		Id:                           1,
		UserId:                       1,
		Username:                     "root",
		AuthIndex:                    "auth-index",
		AuthName:                     "auth.json",
		Enabled:                      true,
		LastRefreshedAt:              1784204160,
		LastUsageTokens:              12345,
		LastUsageQuota:               678,
		LastPlanType:                 "pro",
		LastFiveHourPercent:          21,
		LastFiveHourResetAt:          1783335000,
		LastWeeklyPercent:            34,
		LastWeeklyResetAt:            1783939800,
		LastCodexFiveHourPercent:     55,
		LastCodexFiveHourResetAt:     1783340000,
		LastCodexWeeklyPercent:       67,
		LastCodexWeeklyResetAt:       1783940000,
		LastXAIWeeklyPercent:         45,
		LastXAIWeeklyPeriodStartAt:   1783599360,
		LastXAIWeeklyPeriodEndAt:     1784204160,
		LastXAIProductUsage:          `[{"product":"Api","usage_percent":45}]`,
		LastXAIOnDemandCap:           2500,
		LastXAIOnDemandUsed:          300,
		LastXAIOnDemandUsedRefreshed: true,
		LastXAIBillingPeriodEndAt:    1785542400,
	}).Error)

	binding, err := UpdateCliproxyAuthFileBindingUsage(1, CliproxyUsageRefreshUpdate{
		LastError:               "network timeout",
		PreserveLastRefreshedAt: true,
	})
	require.NoError(t, err)
	require.Equal(t, "network timeout", binding.LastError)
	require.Equal(t, int64(1784204160), binding.LastRefreshedAt)
	require.Equal(t, 12345, binding.LastUsageTokens)
	require.Equal(t, 678, binding.LastUsageQuota)
	require.Equal(t, "pro", binding.LastPlanType)
	require.Equal(t, 21, binding.LastFiveHourPercent)
	require.Equal(t, int64(1783335000), binding.LastFiveHourResetAt)
	require.Equal(t, 34, binding.LastWeeklyPercent)
	require.Equal(t, int64(1783939800), binding.LastWeeklyResetAt)
	require.Equal(t, 55, binding.LastCodexFiveHourPercent)
	require.Equal(t, int64(1783340000), binding.LastCodexFiveHourResetAt)
	require.Equal(t, 67, binding.LastCodexWeeklyPercent)
	require.Equal(t, int64(1783940000), binding.LastCodexWeeklyResetAt)
	require.Equal(t, 45, binding.LastXAIWeeklyPercent)
	require.Equal(t, int64(1783599360), binding.LastXAIWeeklyPeriodStartAt)
	require.Equal(t, int64(1784204160), binding.LastXAIWeeklyPeriodEndAt)
	require.JSONEq(t, `[{"product":"Api","usage_percent":45}]`, binding.LastXAIProductUsage)
	require.Equal(t, 2500, binding.LastXAIOnDemandCap)
	require.Equal(t, 300, binding.LastXAIOnDemandUsed)
	require.True(t, binding.LastXAIOnDemandUsedRefreshed)
	require.Equal(t, int64(1785542400), binding.LastXAIBillingPeriodEndAt)
}

func TestUpdateCliproxyAuthFileBindingUsageAllowsPartialXAIWarning(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))
	DB = db

	require.NoError(t, db.Create(&CliproxyAuthFileBinding{
		Id:              1,
		UserId:          1,
		Username:        "root",
		AuthIndex:       "xai-auth",
		AuthName:        "xai-root@example.com.json",
		Enabled:         true,
		LastUsageTokens: 100,
		LastUsageQuota:  15000,
	}).Error)

	binding, err := UpdateCliproxyAuthFileBindingUsage(1, CliproxyUsageRefreshUpdate{
		LastUsageTokens:            4200,
		LastUsageQuota:             15000,
		LastPlanType:               "SuperGrok",
		LastXAIWeeklyPercent:       45,
		LastXAIWeeklyPeriodStartAt: 1783599360,
		LastXAIWeeklyPeriodEndAt:   1784204160,
		LastXAIProductUsage:        `[{"product":"Api","usage_percent":45}]`,
		LastXAIOnDemandCap:         2500,
		LastXAIOnDemandUsed:        300,
		LastXAIBillingPeriodEndAt:  1785542400,
		LastError:                  "月度额度刷新失败: timeout",
		AllowPartialUsage:          true,
	})
	require.NoError(t, err)
	require.Equal(t, 45, binding.LastXAIWeeklyPercent)
	require.Equal(t, 4200, binding.LastUsageTokens)
	require.Equal(t, "月度额度刷新失败: timeout", binding.LastError)
}
