package middleware

import (
	"log/slog"
	"strings"
	"time"

	"pulsepoll/backend/internal/observability"
	"pulsepoll/backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func RequestContext(logger *slog.Logger, metrics *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 64 {
			requestID, _ = utils.NewEventID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
		metrics.ObserveRequest(c.Writer.Status(), time.Since(started))
		logger.Info("http request",
			"request_id", requestID,
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
			"response_bytes", c.Writer.Size(),
		)
	}
}
