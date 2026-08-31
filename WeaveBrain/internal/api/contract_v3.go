package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
	apiV3Prefix     = "/api/v3"
)

type V3ErrorCode string

const (
	V3ErrorInvalidArgument     V3ErrorCode = "INVALID_ARGUMENT"
	V3ErrorUnauthorized        V3ErrorCode = "UNAUTHORIZED"
	V3ErrorForbidden           V3ErrorCode = "FORBIDDEN"
	V3ErrorNotFound            V3ErrorCode = "NOT_FOUND"
	V3ErrorIdempotencyConflict V3ErrorCode = "IDEMPOTENCY_CONFLICT"
	V3ErrorVersionConflict     V3ErrorCode = "VERSION_CONFLICT"
	V3ErrorFeatureNotEnabled   V3ErrorCode = "FEATURE_NOT_ENABLED"
	V3ErrorPreconditionFailed  V3ErrorCode = "PRECONDITION_FAILED"
	V3ErrorInternal            V3ErrorCode = "INTERNAL"
)

type V3ErrorResponse struct {
	Code      V3ErrorCode    `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Details   map[string]any `json:"details"`
}

type V3MetaResponse struct {
	APIVersion string `json:"api_version"`
	RequestID  string `json:"request_id"`
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}

		c.Set(requestIDKey, requestID)
		c.Header(requestIDHeader, requestID)
		c.Next()
	}
}

func getRequestID(c *gin.Context) string {
	if requestID, ok := c.Get(requestIDKey); ok {
		if value, ok := requestID.(string); ok && value != "" {
			return value
		}
	}

	requestID := uuid.NewString()
	c.Set(requestIDKey, requestID)
	c.Header(requestIDHeader, requestID)
	return requestID
}

func writeV3Error(
	c *gin.Context,
	status int,
	code V3ErrorCode,
	message string,
	details map[string]any,
) {
	if details == nil {
		details = map[string]any{}
	}

	c.AbortWithStatusJSON(status, V3ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: getRequestID(c),
		Details:   details,
	})
}

func registerV3ContractRoutes(engine *gin.Engine) {
	v3 := engine.Group(apiV3Prefix)
	v3.GET("/meta", func(c *gin.Context) {
		c.JSON(http.StatusOK, V3MetaResponse{
			APIVersion: "v3",
			RequestID:  getRequestID(c),
		})
	})

	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, apiV3Prefix+"/") ||
			c.Request.URL.Path == apiV3Prefix {
			writeV3Error(
				c,
				http.StatusNotFound,
				V3ErrorNotFound,
				"resource not found",
				map[string]any{"path": c.Request.URL.Path},
			)
			return
		}

		c.String(http.StatusNotFound, "404 page not found")
	})
}
