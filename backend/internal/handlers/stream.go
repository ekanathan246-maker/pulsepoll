package handlers

import (
	"net/http"
	"time"

	"pulsepoll/backend/internal/realtime"

	"github.com/gin-gonic/gin"
)

// StreamHandler manages SSE connections for live poll updates.
type StreamHandler struct {
	hub *realtime.Hub
}

// NewStreamHandler binds the realtime hub to the stream handler.
func NewStreamHandler(hub *realtime.Hub) *StreamHandler {
	return &StreamHandler{hub: hub}
}

// Stream is an SSE endpoint.
// On connect the client receives a keepalive. Subsequent events are forwarded
// verbatim from Redis pub/sub as `event: update\ndata: {json}\n\n`.
func (s *StreamHandler) Stream(c *gin.Context) {
	slug := c.Param("slug")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Initial keepalive so the client knows the stream is open.
	c.Writer.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ctx := c.Request.Context()

	// Subscribe to the Redis channel for this poll.
	ps := s.hub.Subscribe(ctx, slug)
	defer ps.Close()
	ch := ps.Channel()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.Writer.Write([]byte("event: update\ndata: " + msg.Payload + "\n\n"))
			flusher.Flush()
		case <-ticker.C:
			c.Writer.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}
