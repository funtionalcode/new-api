package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogChannelAccessTest(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMemory, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, db := range []*gorm.DB{mainDB, logDB} {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	require.NoError(t, mainDB.AutoMigrate(&model.Channel{}, &model.User{}, &model.Task{}, &model.Midjourney{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	model.DB, model.LOG_DB = mainDB, logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled, common.RedisEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.MemoryCacheEnabled, common.RedisEnabled = previousMemory, previousRedis
	})
	require.NoError(t, mainDB.Create(&model.User{Id: 10, Username: "viewer"}).Error)
	require.NoError(t, mainDB.Create(&[]model.Channel{
		{Id: 11, Name: "public", Status: common.ChannelStatusEnabled},
		{Id: 12, Name: "restricted-visible", OpenUserIds: model.ChannelOpenUserIds{10}},
		{Id: 13, Name: "restricted-hidden", OpenUserIds: model.ChannelOpenUserIds{20}},
		{Id: 14, Name: "disabled-public", Status: common.ChannelStatusManuallyDisabled},
	}).Error)
	for i, row := range []struct{ channel, quota, tokens, seconds int }{
		{11, 100, 30, 4}, {12, 200, 50, 6}, {13, 900, 999, 99}, {14, 50, 2, 2}, {99, 600, 300, 60},
	} {
		require.NoError(t, logDB.Create(&model.Log{
			UserId: 10, Username: "viewer", TokenId: 501, TokenName: "test-token",
			ChannelId: row.channel, Type: model.LogTypeConsume, CreatedAt: 1700000030 + int64(i),
			Quota: row.quota, PromptTokens: row.tokens, UseTime: row.seconds,
			RequestId: fmt.Sprintf("req-%d", row.channel), UpstreamRequestId: fmt.Sprintf("up-%d", row.channel),
		}).Error)
		require.NoError(t, mainDB.Create(&model.Task{
			TaskID: fmt.Sprintf("task-%d", row.channel), UserId: 10, ChannelId: row.channel,
			Status: model.TaskStatusSuccess, SubmitTime: 1700000030,
		}).Error)
		require.NoError(t, mainDB.Create(&model.Midjourney{
			MjId: fmt.Sprintf("mj-%d", row.channel), UserId: 10, ChannelId: row.channel, SubmitTime: 1700000030,
		}).Error)
	}
}

func runLogAccessRequest(t *testing.T, handler gin.HandlerFunc, path string, userID, role int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", userID)
	c.Set("role", role)
	c.Set("username", "viewer")
	if path == "/api/log/token" {
		c.Set("token_id", 501)
	}
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Params = gin.Params{{Key: "task_id", Value: "task-13"}}
	handler(c)
	return recorder
}

func TestUsageLogsEnforceChannelAccessForAdminsAndUsers(t *testing.T) {
	setupLogChannelAccessTest(t)
	for _, tc := range []struct {
		name                string
		userID, role, total int
		handler             gin.HandlerFunc
	}{
		{"admin", 10, common.RoleAdminUser, 3, GetAllLogs},
		{"root_not_in_allowlist", 1, common.RoleRootUser, 2, GetAllLogs},
		{"root_in_allowlist", 10, common.RoleRootUser, 3, GetAllLogs},
		{"self", 10, common.RoleCommonUser, 3, GetUserLogs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := runLogAccessRequest(t, tc.handler, "/api/log/?type=2&page_size=1&visible_channel_ids=13&user_id=20", tc.userID, tc.role)
			require.Equal(t, http.StatusOK, recorder.Code)
			var result struct {
				Success bool `json:"success"`
				Data    struct {
					Total int         `json:"total"`
					Items []model.Log `json:"items"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
			require.True(t, result.Success)
			assert.Equal(t, tc.total, result.Data.Total)
			require.Len(t, result.Data.Items, 1)
			assert.Equal(t, 14, result.Data.Items[0].ChannelId)
		})
	}
}

func TestUsageLogFiltersCannotSelectHiddenChannels(t *testing.T) {
	setupLogChannelAccessTest(t)
	for _, filter := range []string{"channel=13", "channel_name=restricted-hidden", "request_id=req-13", "upstream_request_id=up-13", "channel=99"} {
		t.Run(filter, func(t *testing.T) {
			for _, handler := range []gin.HandlerFunc{GetAllLogs, GetLogsStat} {
				recorder := runLogAccessRequest(t, handler, "/api/log/?type=2&"+filter, 10, common.RoleRootUser)
				var result struct {
					Success bool `json:"success"`
					Data    struct {
						Total int         `json:"total"`
						Items []model.Log `json:"items"`
						Quota int         `json:"quota"`
						TPM   int         `json:"tpm"`
						Count int         `json:"avg_use_time_count"`
					} `json:"data"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
				require.True(t, result.Success)
				assert.Zero(t, result.Data.Total)
				assert.Empty(t, result.Data.Items)
				assert.Zero(t, result.Data.Quota)
				assert.Zero(t, result.Data.TPM)
				assert.Zero(t, result.Data.Count)
			}
		})
	}
}

func TestUsageLogStatsAndTokenReadsRespectRevokedAccess(t *testing.T) {
	setupLogChannelAccessTest(t)
	for _, handler := range []gin.HandlerFunc{GetLogsStat, GetLogsSelfStat} {
		recorder := runLogAccessRequest(t, handler, "/api/log/stat?type=2&start_timestamp=1700000000&end_timestamp=1700000060", 10, common.RoleRootUser)
		var result struct {
			Success bool       `json:"success"`
			Data    model.Stat `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
		require.True(t, result.Success)
		assert.Equal(t, 350, result.Data.Quota)
		assert.Equal(t, 3, result.Data.Rpm)
		assert.Equal(t, 82, result.Data.Tpm)
		assert.Equal(t, 4.0, result.Data.AvgUseTime)
		assert.Equal(t, int64(3), result.Data.AvgUseTimeCount)
	}
	for _, revoked := range []bool{false, true} {
		if revoked {
			require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 12).Update("open_user_ids", model.ChannelOpenUserIds{20}).Error)
		}
		recorder := runLogAccessRequest(t, GetLogByKey, "/api/log/token", 10, common.RoleRootUser)
		var result struct {
			Success bool        `json:"success"`
			Data    []model.Log `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
		require.True(t, result.Success)
		ids := make([]int, 0, len(result.Data))
		for _, item := range result.Data {
			ids = append(ids, item.ChannelId)
		}
		if revoked {
			assert.ElementsMatch(t, []int{11, 14}, ids)
		} else {
			assert.ElementsMatch(t, []int{11, 12, 14}, ids)
		}
	}
}

func TestTaskAndDrawingLogsEnforceChannelAccess(t *testing.T) {
	setupLogChannelAccessTest(t)
	for _, handler := range []gin.HandlerFunc{GetAllTask, GetUserTask, GetAllMidjourney, GetUserMidjourney} {
		recorder := runLogAccessRequest(t, handler, "/api/task?page_size=1", 10, common.RoleRootUser)
		var result struct {
			Success bool `json:"success"`
			Data    struct {
				Total int              `json:"total"`
				Items []map[string]any `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
		require.True(t, result.Success)
		assert.Equal(t, 3, result.Data.Total)
		assert.Len(t, result.Data.Items, 1)
		assert.NotContains(t, recorder.Body.String(), "task-13")
		assert.NotContains(t, recorder.Body.String(), "mj-13")
	}
	recorder := runLogAccessRequest(t, GetDashboardTaskArtifacts, "/api/task/task-13/artifacts", 10, common.RoleRootUser)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "content_url")
}

func TestUsageLogAccessFailsClosedAndKeepsUnassignedLogs(t *testing.T) {
	setupLogChannelAccessTest(t)
	require.NoError(t, model.LOG_DB.Create(&model.Log{UserId: 10, TokenId: 501, Type: model.LogTypeTopup, ChannelId: 0}).Error)
	recorder := runLogAccessRequest(t, GetUserLogs, "/api/log/self?type=1", 10, common.RoleCommonUser)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.Data.Total)
	require.NoError(t, model.DB.Migrator().DropTable(&model.Channel{}))
	for _, handler := range []gin.HandlerFunc{GetAllLogs, GetUserLogs, GetLogByKey, GetLogsStat, GetLogsSelfStat, GetAllTask, GetUserTask, GetAllMidjourney, GetUserMidjourney} {
		recorder = runLogAccessRequest(t, handler, "/api/log/", 10, common.RoleRootUser)
		var failed struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &failed))
		assert.False(t, failed.Success)
		assert.NotContains(t, recorder.Body.String(), "req-13")
	}
}
