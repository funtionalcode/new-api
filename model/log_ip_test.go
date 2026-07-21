package model

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordLogPersistsClientIP(t *testing.T) {
	require.NoError(t, LOG_DB.Where("user_id = ?", 9001).Delete(&Log{}).Error)

	RecordLog(9001, LogTypeSystem, "unit test system op", "198.51.100.20")

	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", 9001, LogTypeSystem).Order("id desc").First(&log).Error)
	assert.Equal(t, "198.51.100.20", log.Ip)
	assert.Equal(t, "unit test system op", log.Content)
}

func TestRecordConsumeLogAlwaysRecordsIPEvenWhenOptionDisabled(t *testing.T) {
	require.NoError(t, LOG_DB.Where("user_id = ?", 9002).Delete(&Log{}).Error)

	original := common.RecordIpLogEnabled
	common.RecordIpLogEnabled = false
	t.Cleanup(func() {
		common.RecordIpLogEnabled = original
	})

	originalTrusted := common.TrustedProxyCIDRs
	common.TrustedProxyCIDRs = "172.16.0.0/12"
	t.Cleanup(func() {
		common.TrustedProxyCIDRs = originalTrusted
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "172.22.0.1:51234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	c.Request = req
	c.Set("username", "ip-user")

	RecordConsumeLog(c, 9002, RecordConsumeLogParams{
		ModelName: "gpt-test",
		TokenName: "tok",
		Quota:     1,
		Content:   "consume with ip",
	})

	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", 9002, LogTypeConsume).Order("id desc").First(&log).Error)
	assert.Equal(t, "203.0.113.50", log.Ip)
}
