package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateVolcengineQuotaBindingUsagePreservesLastSuccessOnFailure(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&VolcengineQuotaBinding{}))
	DB = db

	binding := &VolcengineQuotaBinding{
		Name:                    "Agent Plan",
		RequestCurl:             "curl saved",
		LastPlanType:            "medium",
		LastFiveHourQuota:       10_000,
		LastFiveHourUsedAFP:     0.3736,
		LastFiveHourSubscribeAt: 1_785_746_215,
		LastFiveHourResetAt:     1_785_764_215,
		Enabled:                 true,
	}
	require.NoError(t, CreateVolcengineQuotaBinding(binding))

	updated, err := UpdateVolcengineQuotaBindingUsage(binding.Id, VolcengineQuotaUsageRefreshUpdate{
		LastFiveHourQuota:   99,
		LastFiveHourUsedAFP: 99,
		LastError:           "InvalidCSRFToken",
	})
	require.NoError(t, err)
	assert.Equal(t, "InvalidCSRFToken", updated.LastError)
	assert.Equal(t, "medium", updated.LastPlanType)
	assert.Equal(t, int64(10_000), updated.LastFiveHourQuota)
	assert.InDelta(t, 0.3736, updated.LastFiveHourUsedAFP, 0.000001)
	assert.Equal(t, int64(1_785_764_215), updated.LastFiveHourResetAt)
	assert.Positive(t, updated.LastRefreshedAt)
}
