package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/observability"
	"pulsepoll/backend/internal/realtime"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	writeWait                 = 5 * time.Second
	pongWait                  = 45 * time.Second
	pingEvery                 = 20 * time.Second
	maxWebSocketClients int64 = 2000
)

type StreamHandler struct {
	hub      *realtime.Hub
	polls    *mongo.Collection
	votes    *mongo.Collection
	upgrader websocket.Upgrader
	shutdown context.Context
	metrics  *observability.Metrics
}

func NewStreamHandler(hub *realtime.Hub, db *mongo.Database, shutdown context.Context, metrics *observability.Metrics) *StreamHandler {
	return &StreamHandler{
		hub:      hub,
		polls:    db.Collection("polls"),
		votes:    db.Collection("votes"),
		shutdown: shutdown,
		metrics:  metrics,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 5 * time.Second,
			ReadBufferSize:   1024, WriteBufferSize: 2048,
			CheckOrigin: func(*http.Request) bool { return true }, // exact origin is checked by CORS middleware first
		},
	}
}

// Stream upgrades to a bounded WebSocket, sends a durable snapshot first, and
// then forwards versioned Redis events. Clients replace state from REST on any
// version gap or reconnect.
func (s *StreamHandler) Stream(c *gin.Context) {
	if !s.metrics.TryOpenWebSocket(maxWebSocketClients) {
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("stream_capacity", "Live updates are at capacity. Please retry shortly."))
		return
	}
	defer s.metrics.CloseWebSocket()

	slug := c.Param("slug")
	var poll models.Poll
	if err := s.polls.FindOne(c.Request.Context(), bson.M{"slug": slug, "deleted_at": bson.M{"$exists": false}}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, middleware.ErrorBody("poll_not_found", "Poll not found."))
		return
	}

	// Subscribe before taking the snapshot. Redis buffers events that arrive
	// while Mongo is read, eliminating the snapshot->subscribe loss window.
	ctx := c.Request.Context()
	pubsub := s.hub.Subscribe(ctx, slug)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("stream_unavailable", "Live updates are temporarily unavailable."))
		return
	}
	if err := s.polls.FindOne(ctx, bson.M{"slug": slug, "deleted_at": bson.M{"$exists": false}}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, middleware.ErrorBody("poll_not_found", "Poll not found."))
		return
	}
	messages := pubsub.Channel(redis.WithChannelSize(256))

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	ownerID, ownerAuthenticated := c.Get("user_id")
	canViewResults := ownerAuthenticated && ownerID == poll.CreatedBy
	canViewResults = canViewResults || poll.Closed || !poll.HideResults || (poll.ClosesAt != nil && !poll.ClosesAt.After(time.Now().UTC()))
	if !canViewResults {
		if voterID, cookieErr := c.Cookie("ppv"); cookieErr == nil && voterID != "" {
			count, countErr := s.votes.CountDocuments(c.Request.Context(), bson.M{"poll_id": poll.ID, "voter_id": voterID})
			canViewResults = countErr == nil && count > 0
		}
	}
	counts, total := countsFromPoll(&poll)
	if !canViewResults {
		counts = nil
		total = 0
	}
	if err := conn.WriteJSON(gin.H{
		"type": "snapshot", "pollId": slug, "version": poll.Version,
		"counts": counts, "total": total, "status": pollStatus(&poll),
	}); err != nil {
		return
	}

	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown.Done():
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server restarting"),
				time.Now().Add(writeWait))
			return
		case <-done:
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			payload := []byte(message.Payload)
			if !canViewResults {
				var event map[string]any
				if json.Unmarshal(payload, &event) == nil {
					delete(event, "counts")
					delete(event, "optionId")
					delete(event, "delta")
					payload, _ = json.Marshal(event)
				}
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}
