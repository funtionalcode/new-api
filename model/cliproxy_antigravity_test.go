package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCliproxyAntigravitySnapshotSurvivesEditAndFailedRefresh(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, db.AutoMigrate(&legacyCliproxyAuthFileBinding{}))
	require.NoError(t, db.Create(&legacyCliproxyAuthFileBinding{Id: 1, UserId: 1, AuthIndex: "ag", AuthName: "antigravity-test.json", Enabled: true}).Error)
	require.NoError(t, db.AutoMigrate(&CliproxyAuthFileBinding{}))
	_, err = UpdateCliproxyAuthFileBinding(1, CliproxyAuthFileBindingUpdate{UserId: 1, AuthIndex: "ag", AuthName: "account.json", Provider: "antigravity", Enabled: true})
	require.NoError(t, err)
	snapshot := `[{"bucket_id":"gemini-5h","remaining_fraction":0.9917125,"reset_at":1789038951}]`
	_, err = UpdateCliproxyAuthFileBindingUsage(1, CliproxyUsageRefreshUpdate{LastAntigravityQuota: snapshot})
	require.NoError(t, err)
	_, err = UpdateCliproxyAuthFileBinding(1, CliproxyAuthFileBindingUpdate{UserId: 1, AuthIndex: "ag", AuthName: "account.json", Note: "已更新备注", Enabled: true})
	require.NoError(t, err)
	_, err = UpdateCliproxyAuthFileBindingUsage(1, CliproxyUsageRefreshUpdate{LastError: "upstream unavailable"})
	require.NoError(t, err)
	binding, err := GetCliproxyAuthFileBindingById(1)
	require.NoError(t, err)
	assert.Equal(t, "antigravity", binding.Provider)
	assert.Equal(t, snapshot, binding.LastAntigravityQuota)
	assert.Equal(t, "upstream unavailable", binding.LastError)
}
