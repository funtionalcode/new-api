package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
