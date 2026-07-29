package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// BackupAgentAuth authenticates host-side backup agent callbacks via a shared
// secret. Empty expected or provided tokens are rejected so a missing env
// configuration cannot become an open endpoint.
func BackupAgentAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := common.GetEnvOrDefaultString("DB_BACKUP_AGENT_TOKEN", "")
		provided := c.GetHeader("X-Backup-Agent-Token")
		if expected == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "unauthorized",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
