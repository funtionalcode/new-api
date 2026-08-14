package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionPersistsMultiDeviceLoginSetting(t *testing.T) {
	previousDB := DB
	previousOptionMap := common.OptionMap
	previousEnabled := common.IsMultiDeviceLoginEnabled()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	common.OptionMap = map[string]string{}
	common.SetMultiDeviceLoginEnabled(false)
	t.Cleanup(func() {
		DB = previousDB
		common.OptionMap = previousOptionMap
		common.SetMultiDeviceLoginEnabled(previousEnabled)
	})

	require.NoError(t, UpdateOption("MultiDeviceLoginEnabled", "true"))
	assert.True(t, common.IsMultiDeviceLoginEnabled())
	assert.Equal(t, "true", common.OptionMap["MultiDeviceLoginEnabled"])

	var option Option
	require.NoError(t, db.Where("key = ?", "MultiDeviceLoginEnabled").First(&option).Error)
	assert.Equal(t, "true", option.Value)

	require.NoError(t, UpdateOption("MultiDeviceLoginEnabled", "false"))
	assert.False(t, common.IsMultiDeviceLoginEnabled())
}
