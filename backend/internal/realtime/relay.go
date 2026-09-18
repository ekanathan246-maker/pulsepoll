package realtime

import (
	"context"
	"log/slog"
	"math"
	"time"

	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/observability"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Relay struct {
	outbox  *mongo.Collection
	hub     *Hub
	log     *slog.Logger
	metrics *observability.Metrics
}

func NewRelay(db *mongo.Database, hub *Hub, logger *slog.Logger, metrics *observability.Metrics) *Relay {
	return &Relay{outbox: db.Collection("outbox"), hub: hub, log: logger, metrics: metrics}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < 20; i++ {
				processed, err := r.processOne(ctx)
				if err != nil {
					r.log.Error("outbox relay", "error", err)
					break
				}
				if !processed {
					break
				}
			}
		}
	}
}

func (r *Relay) processOne(ctx context.Context) (bool, error) {
	now := time.Now().UTC()
	filter := bson.M{
		"processed_at":    bson.M{"$exists": false},
		"next_attempt_at": bson.M{"$lte": now},
		"$or": bson.A{
			bson.M{"claimed_until": bson.M{"$exists": false}},
			bson.M{"claimed_until": bson.M{"$lt": now}},
		},
	}
	claimUntil := now.Add(30 * time.Second)
	update := bson.M{"$set": bson.M{"claimed_until": claimUntil}, "$inc": bson.M{"attempts": 1}}
	var event models.OutboxEvent
	err := r.outbox.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().
		// A single relay publishes the durable per-poll sequence in version order.
		// Sorting by request time is unsafe: concurrent transactions can commit in
		// a different order and make clients observe version 9 before version 8.
		SetSort(bson.D{{Key: "aggregate_id", Value: 1}, {Key: "version", Value: 1}, {Key: "created_at", Value: 1}}).
		SetReturnDocument(options.After)).Decode(&event)
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := r.hub.ApplyEvent(ctx, event); err != nil {
		r.metrics.RelayFailed()
		seconds := math.Min(30, math.Pow(2, float64(event.Attempts)))
		_, updateErr := r.outbox.UpdateByID(ctx, event.ID, bson.M{
			"$set":   bson.M{"next_attempt_at": now.Add(time.Duration(seconds) * time.Second)},
			"$unset": bson.M{"claimed_until": ""},
		})
		if updateErr != nil {
			return true, updateErr
		}
		return true, err
	}
	_, err = r.outbox.UpdateByID(ctx, event.ID, bson.M{
		"$set":   bson.M{"processed_at": now},
		"$unset": bson.M{"claimed_until": ""},
	})
	if err == nil {
		r.metrics.RelayProcessed()
	}
	return true, err
}
