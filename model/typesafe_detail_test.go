package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTypeSafeDetailOwnershipAndChannelAccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previous := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previous; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&Log{}))
	input := `{"model":"jev-latest","state":{"request":"Write a greeting"},"questions":{"check":{"type":"noul","instructions":"Is this a greeting?"}}}`
	output := `{"answers":{"check":{"noul":0.9,"confidence":0.95}}}`
	for _, tc := range []struct {
		name                                   string
		userID                                 int
		channels                               []int
		childUser, childChannel, parentChannel int
		parentLink, stageLink                  string
		legacy, missingSnapshot, denied        bool
	}{
		{name: "own linked exchange", userID: 1, channels: []int{18, 21}, childUser: 1, childChannel: 21, parentChannel: 18, parentLink: "parent", stageLink: "before"},
		{name: "other user", userID: 2, channels: []int{18, 21}, denied: true},
		{name: "parent channel hidden", userID: 1, channels: []int{21}, denied: true},
		{name: "evaluation channel hidden", userID: 1, channels: []int{18}, denied: true},
		{name: "no channels", userID: 1, denied: true},
		{name: "child belongs to another user", userID: 1, channels: []int{18, 21}, childUser: 2, childChannel: 21, parentChannel: 18, parentLink: "parent", stageLink: "before", denied: true},
		{name: "child channel differs", userID: 1, channels: []int{18, 21, 22}, childUser: 1, childChannel: 22, denied: true},
		{name: "child linked to another request", userID: 1, channels: []int{18, 21}, childUser: 1, childChannel: 21, parentChannel: 18, parentLink: "another", stageLink: "before", denied: true},
		{name: "child linked to another stage", userID: 1, channels: []int{18, 21}, childUser: 1, childChannel: 21, parentChannel: 18, parentLink: "parent", stageLink: "after", denied: true},
		{name: "child linked to another parent channel", userID: 1, channels: []int{18, 21}, childUser: 1, childChannel: 21, parentChannel: 19, parentLink: "parent", stageLink: "before", denied: true},
		{name: "legacy summary only", userID: 1, channels: []int{18, 21}, legacy: true},
		{name: "legacy child without snapshot", userID: 1, channels: []int{18, 21}, childUser: 1, childChannel: 21, parentChannel: 18, parentLink: "parent", stageLink: "before", missingSnapshot: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Where("1 = 1").Delete(&Log{}).Error)
			result := map[string]any{"stage": "before", "status": "success", "model": "jev-latest", "channel_id": 21, "request_id": "child", "truncated": true, "answers": map[string]any{"check": map[string]any{"noul": 0.9}}}
			if tc.legacy {
				delete(result, "request_id")
			}
			parent := &Log{UserId: 1, RequestId: "parent", ChannelId: 18, Type: LogTypeConsume, Other: common.MapToJsonStr(map[string]any{"admin_info": map[string]any{"typesafe": []any{result}, "secret": "not returned"}})}
			childAdmin := map[string]any{"typesafe": []any{map[string]any{"stage": tc.stageLink, "parent_request_id": tc.parentLink, "parent_channel_id": tc.parentChannel}}}
			if !tc.missingSnapshot {
				childAdmin["typesafe_exchange"] = map[string]any{"request": map[string]any{"body": input, "bytes": len(input)}, "response": map[string]any{"body": output, "bytes": len(output)}}
			}
			child := &Log{UserId: tc.childUser, RequestId: "child", ChannelId: tc.childChannel, Type: LogTypeConsume, Other: common.MapToJsonStr(map[string]any{"admin_info": childAdmin})}
			require.NoError(t, db.Create(parent).Error)
			require.NoError(t, db.Create(child).Error)
			detail, err := GetUserTypeSafeEvaluationDetail(tc.userID, "parent", "before", tc.channels)
			if tc.denied {
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				assert.Nil(t, detail)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, detail)
			assert.Equal(t, "jev-latest", detail.Model)
			assert.True(t, detail.Truncated)
			assert.Equal(t, map[string]any{"check": map[string]any{"noul": 0.9}}, detail.Answers)
			if tc.legacy || tc.missingSnapshot {
				assert.Nil(t, detail.Request)
				assert.Nil(t, detail.Response)
			} else {
				require.NotNil(t, detail.Request)
				require.NotNil(t, detail.Response)
				assert.Equal(t, input, detail.Request.Body)
				assert.Equal(t, output, detail.Response.Body)
			}
			body, err := common.Marshal(detail)
			require.NoError(t, err)
			assert.NotContains(t, string(body), "admin_info")
			assert.NotContains(t, string(body), "channel_id")
			assert.NotContains(t, string(body), "not returned")
		})
	}
}
