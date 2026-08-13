package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORSExposesCursorAgentID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.GET("/chat", func(c *gin.Context) {
		c.Header("X-Cursor-Agent-ID", "bc-session")
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/chat", nil)
	request.Header.Set("Origin", "https://chat.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	exposed := strings.ToLower(strings.Join(response.Header().Values("Access-Control-Expose-Headers"), ","))
	assert.Contains(t, exposed, "x-cursor-agent-id")
	assert.Contains(t, exposed, "x-cursor-agent-signature")
}
