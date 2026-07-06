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
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))
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
