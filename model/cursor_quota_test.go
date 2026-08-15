package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateCursorQuotaBindingUsagePreservesLastSuccessOnFailure(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CursorQuotaBinding{}))
	DB = db

	binding := &CursorQuotaBinding{
		Name:                    "Cursor Ultra",
		RequestCurl:             "curl period",
		UsageAmountCurl:         "curl aggregated",
		UsageCostCurl:           "curl plan",
		LastPlanName:            "Ultra",
		LastPlanLimitCents:      40_000,
		LastPlanAPIPercent:      10.5,
		LastTotalInputTokens:    300,
		LastBillingCycleStartAt: 1_786_579_200,
		Enabled:                 true,
	}
	require.NoError(t, CreateCursorQuotaBinding(binding))

	updated, err := UpdateCursorQuotaBindingUsage(binding.Id, CursorQuotaUsageRefreshUpdate{
		LastPlanName:         "incorrect",
		LastPlanLimitCents:   1,
		LastTotalInputTokens: 1,
		LastError:            "session expired",
	})
	require.NoError(t, err)
	assert.Equal(t, "session expired", updated.LastError)
	assert.Equal(t, "Ultra", updated.LastPlanName)
	assert.InDelta(t, 40_000, updated.LastPlanLimitCents, 0.000001)
	assert.InDelta(t, 10.5, updated.LastPlanAPIPercent, 0.000001)
	assert.Equal(t, int64(300), updated.LastTotalInputTokens)
	assert.Equal(t, int64(1_786_579_200), updated.LastBillingCycleStartAt)
	assert.Positive(t, updated.LastRefreshedAt)
}
