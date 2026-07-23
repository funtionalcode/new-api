package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetQuotaDataGroupByUserIncludesRemarkByUserID(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{
		Id:       1,
		Username: "current-name",
		Password: "password",
		Remark:   "核心用户",
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:    1,
		Username:  "legacy-name",
		ModelName: "gpt-4.1",
		CreatedAt: 1000,
		Count:     2,
		Quota:     30,
		TokenUsed: 40,
	}).Error)

	rows, err := GetQuotaDataGroupByUser(900, 1100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].UserID)
	assert.Equal(t, "legacy-name", rows[0].Username)
	assert.Equal(t, "核心用户", rows[0].Remark)
}
