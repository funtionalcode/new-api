package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDistributorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}))

	t.Cleanup(func() {
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestIsModelAllowedByUserAllowsHistoricalUnlimitedTokenModel(t *testing.T) {
	db := setupDistributorTestDB(t)
	settingBytes, err := common.Marshal(dto.UserSetting{
		ModelLimitsEnabled: true,
		ModelLimits:        []string{"allowed-model"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Id:       3001,
		Username: "legacy-token-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Setting:  string(settingBytes),
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 3001)
	ctx.Set("token_model_limit_enabled", false)

	require.True(t, isModelAllowedByUser(ctx, "allowed-model"))
	require.False(t, isModelAllowedByUser(ctx, "denied-model"))
}

func TestIsModelAllowedByUserSkipsLimitWhenUserLimitDisabled(t *testing.T) {
	db := setupDistributorTestDB(t)
	settingBytes, err := common.Marshal(dto.UserSetting{
		ModelLimitsEnabled: false,
		ModelLimits:        []string{"allowed-model"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{
		Id:       3002,
		Username: "unlimited-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Setting:  string(settingBytes),
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 3002)

	require.True(t, isModelAllowedByUser(ctx, "any-model"))
}

func TestGetModelRequestDefaultsSTTModelWithoutModelField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("url", "https://example.com/audio.mp3"))
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/stt", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	modelRequest, shouldSelectChannel, err := getModelRequest(ctx)

	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.Equal(t, "grok-stt", modelRequest.Model)
	relayMode, exists := ctx.Get("relay_mode")
	require.True(t, exists)
	require.Equal(t, relayconstant.RelayModeAudioTranscription, relayMode)
}

func TestGetModelRequestReadsPlaygroundAudioTranscriptionMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "volc-asr-2"))
	require.NoError(t, writer.WriteField("group", "vip"))
	require.NoError(t, writer.WriteField("url", "https://example.com/audio.mp3"))
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/audio/transcriptions", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	modelRequest, shouldSelectChannel, err := getModelRequest(ctx)

	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.Equal(t, "volc-asr-2", modelRequest.Model)
	require.Equal(t, "vip", modelRequest.Group)
	relayMode, exists := ctx.Get("relay_mode")
	require.True(t, exists)
	require.Equal(t, relayconstant.RelayModeAudioTranscription, relayMode)
}
