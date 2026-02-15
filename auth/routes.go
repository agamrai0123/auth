package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func routes(r *gin.Engine, s *authServer) {
	service := r.Group("auth-server")
	api := service.Group("/v1")
	v1 := api.Group("/oauth")
	v1.POST("/token", s.tokenHandler)
	v1.POST("/ott", s.ottHandler)
	v1.POST("/validate", s.validateHandler)
	v1.POST("/revoke", s.revokeHandler)
	v1.GET("/", func(c *gin.Context) {
		c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload") // HSTS
		c.String(http.StatusOK, "ok")
	})
}
