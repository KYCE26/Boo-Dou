package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health reports only whether a key is set, never the key itself, so the UI
// can warn early without the secret leaving the server.
func Health(hasAPIKey bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "ok",
			"llm_configured": hasAPIKey,
		})
	}
}
