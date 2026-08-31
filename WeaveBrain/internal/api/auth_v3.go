package api

import (
	"net/http"
	"strings"

	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
)

func v3JWTMiddleware(cfg auth.TokenConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeV3Error(
				c,
				http.StatusUnauthorized,
				V3ErrorUnauthorized,
				"missing or invalid authorization header",
				nil,
			)
			return
		}

		claims, err := auth.ValidateToken(cfg, parts[1])
		if err != nil {
			writeV3Error(
				c,
				http.StatusUnauthorized,
				V3ErrorUnauthorized,
				"invalid or expired token",
				nil,
			)
			return
		}

		SetCurrentUserID(c, claims.UserID)
		c.Next()
	}
}
