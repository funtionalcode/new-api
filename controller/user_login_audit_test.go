package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordLoginAuditUsesConfiguredRealClientIP(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	originalTrustedProxyCIDRs := common.TrustedProxyCIDRs
	common.TrustedProxyCIDRs = "172.16.0.0/12"
	t.Cleanup(func() {
		common.TrustedProxyCIDRs = originalTrustedProxyCIDRs
	})

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies([]string{"127.0.0.1"}))

	user := &model.User{Id: 1001, Username: "login-user"}
	router.POST("/api/user/login", func(c *gin.Context) {
		recordLoginAudit(user, c)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	req.RemoteAddr = "172.22.0.1:51234"
	req.Header.Set("X-Forwarded-For", "198.51.100.10, 172.22.0.1")
	req.Header.Set("X-Real-IP", "203.0.113.20")
	req.Header.Set("User-Agent", "login-audit-test")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)

	var log model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLogin).First(&log).Error)
	assert.Equal(t, "198.51.100.10", log.Ip)
	assert.Equal(t, "login-user", log.Username)
}
