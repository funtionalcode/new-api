package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupAgentAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("rejects empty expected token", func(t *testing.T) {
		t.Setenv("DB_BACKUP_AGENT_TOKEN", "")
		router := gin.New()
		router.POST("/pending", BackupAgentAuth(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, "/pending", nil)
		req.Header.Set("X-Backup-Agent-Token", "anything")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects empty provided token", func(t *testing.T) {
		t.Setenv("DB_BACKUP_AGENT_TOKEN", "secret-token")
		router := gin.New()
		router.POST("/pending", BackupAgentAuth(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, "/pending", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects wrong token", func(t *testing.T) {
		t.Setenv("DB_BACKUP_AGENT_TOKEN", "secret-token")
		router := gin.New()
		router.POST("/pending", BackupAgentAuth(), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, "/pending", nil)
		req.Header.Set("X-Backup-Agent-Token", "wrong-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("accepts matching token", func(t *testing.T) {
		t.Setenv("DB_BACKUP_AGENT_TOKEN", "secret-token")
		router := gin.New()
		router.POST("/pending", BackupAgentAuth(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		})

		req := httptest.NewRequest(http.MethodPost, "/pending", nil)
		req.Header.Set("X-Backup-Agent-Token", "secret-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})
}
