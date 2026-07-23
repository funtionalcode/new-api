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
	assert.Equal(t, "gpt-4.1", rows[0].ModelName)
	assert.Equal(t, "核心用户", rows[0].Remark)
}

func TestGetQuotaDataGroupByUserKeepsModelDimension(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{
		Id:       1,
		Username: "alice",
		Password: "password",
		Remark:   "核心用户",
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:    1,
		Username:  "alice",
		ModelName: "gpt-4.1",
		CreatedAt: 1000,
		Count:     2,
		Quota:     30,
		TokenUsed: 40,
	}).Error)
	require.NoError(t, DB.Create(&QuotaData{
		UserID:    1,
		Username:  "alice",
		ModelName: "claude-sonnet-4",
		CreatedAt: 1000,
		Count:     3,
		Quota:     50,
		TokenUsed: 60,
	}).Error)

	rows, err := GetQuotaDataGroupByUser(900, 1100)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	rowsByModel := map[string]*QuotaData{}
	for _, row := range rows {
		rowsByModel[row.ModelName] = row
	}
	require.Contains(t, rowsByModel, "gpt-4.1")
	require.Contains(t, rowsByModel, "claude-sonnet-4")
	assert.Equal(t, 40, rowsByModel["gpt-4.1"].TokenUsed)
	assert.Equal(t, 60, rowsByModel["claude-sonnet-4"].TokenUsed)
	assert.Equal(t, "核心用户", rowsByModel["gpt-4.1"].Remark)
}
