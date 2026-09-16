package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Hub wraps Redis operations for live vote counts and pub/sub fan-out.
type Hub struct {
	rdb *redis.Client
}

// NewHub returns a Hub bound to the given Redis client.
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{rdb: rdb}
}

// VoteSnapshot is the JSON payload sent on each vote event.
type VoteSnapshot struct {
	Counts map[string]int64 `json:"counts"`
	Total  int64            `json:"total"`
	Closed bool             `json:"closed"`
}

// IncrVote atomically bumps one option, bumps total, inserts the voter id
// into the dedup set (returning true if the vote was new), then publishes
// a full snapshot to the pub/sub channel.
func (h *Hub) IncrVote(ctx context.Context, slug, optionID, voterID string) (*VoteSnapshot, bool, error) {
	votesKey := PollVotesKey(slug)
	totalKey := PollTotalKey(slug)
	votersKey := PollVotersKey(slug)

	pipe := h.rdb.Pipeline()
	pipe.HIncrBy(ctx, votesKey, optionID, 1)
	pipe.Incr(ctx, totalKey)
	isNewVoter := pipe.SAdd(ctx, votersKey, voterID)
	pipe.Expire(ctx, votesKey, 30*24*time.Hour)
	pipe.Expire(ctx, totalKey, 30*24*time.Hour)
	pipe.Expire(ctx, votersKey, 30*24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, false, err
	}
	// SAdd returns the number of elements actually added (0 = already existed).
	if isNewVoter.Val() == 0 {
		// Already voted – undo the counts we just bumped.
		h.rdb.HIncrBy(ctx, votesKey, optionID, -1)
		h.rdb.IncrBy(ctx, totalKey, -1)
		return nil, false, nil
	}

	counts, total, err := h.SnapshotCounts(ctx, slug)
	if err != nil {
		return nil, false, err
	}
	snap := &VoteSnapshot{Counts: counts, Total: total}

	payload, err := json.Marshal(snap)
	if err != nil {
		return nil, true, err
	}
	if err := h.rdb.Publish(ctx, PollEventsChannel(slug), string(payload)).Err(); err != nil {
		log.Printf("publish error: %v", err)
	}
	return snap, true, nil
}

// SnapshotCounts reads the full vote tallies straight from Redis.
func (h *Hub) SnapshotCounts(ctx context.Context, slug string) (map[string]int64, int64, error) {
	counts, err := h.rdb.HGetAll(ctx, PollVotesKey(slug)).Result()
	if err != nil {
		return nil, 0, err
	}
	out := make(map[string]int64, len(counts))
	for k, v := range counts {
		var n int64
		fmt.Sscanf(v, "%d", &n)
		out[k] = n
	}
	total, err := h.rdb.Get(ctx, PollTotalKey(slug)).Int64()
	if err != nil && err != redis.Nil {
		return out, 0, nil // total key not yet set is fine
	}
	return out, total, nil
}

// PublishClosed pushes a marker event so viewers instantly see the poll is closed.
func (h *Hub) PublishClosed(ctx context.Context, slug string) error {
	snap := &VoteSnapshot{Closed: true}
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return h.rdb.Publish(ctx, PollEventsChannel(slug), string(payload)).Err()
}

// Subscribe returns a Redis pub/sub subscription for a poll's event channel.
func (h *Hub) Subscribe(ctx context.Context, slug string) *redis.PubSub {
	return h.rdb.Subscribe(ctx, PollEventsChannel(slug))
}
