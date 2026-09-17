package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User is a registered account that can create and manage polls.
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

// Session stores only hashes of browser credentials. A leaked database cannot
// be used to replay a session or CSRF token.
type Session struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	TokenHash string             `bson:"token_hash" json:"-"`
	CSRFHash  string             `bson:"csrf_hash" json:"-"`
	UserID    primitive.ObjectID `bson:"user_id" json:"-"`
	ExpiresAt time.Time          `bson:"expires_at" json:"-"`
	CreatedAt time.Time          `bson:"created_at" json:"-"`
}

// Option is a single answerable choice on a poll. The Count is the durable
// tally persisted in Mongo; live counts live in Redis.
type Option struct {
	ID    string `bson:"id" json:"id"`
	Text  string `bson:"text" json:"text"`
	Count int64  `bson:"count" json:"count"`
}

// Poll is the definition of a poll (the persistent source of truth).
type Poll struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Slug        string             `bson:"slug" json:"slug"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description"`
	Options     []Option           `bson:"options" json:"options"`
	CreatedBy   primitive.ObjectID `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
	Closed      bool               `bson:"closed" json:"closed"`
	HideResults bool               `bson:"hide_results" json:"hide_results"`
	Version     int64              `bson:"version" json:"version"`
	ClosesAt    *time.Time         `bson:"closes_at,omitempty" json:"closes_at,omitempty"`
	DeletedAt   *time.Time         `bson:"deleted_at,omitempty" json:"-"`
}

// Vote records a single ballot, kept as an append-only ledger in Mongo.
type Vote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PollID    primitive.ObjectID `bson:"poll_id" json:"poll_id"`
	OptionID  string             `bson:"option_id" json:"option_id"`
	VoterID   string             `bson:"voter_id" json:"voter_id"`
	UserAgent string             `bson:"user_agent,omitempty" json:"-"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// OutboxEvent is committed beside the durable state mutation. A relay applies
// it to Redis at least once; Redis de-duplicates by EventID.
type OutboxEvent struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	EventID       string             `bson:"event_id" json:"eventId"`
	AggregateID   string             `bson:"aggregate_id" json:"pollId"`
	Type          string             `bson:"type" json:"type"`
	Version       int64              `bson:"version" json:"version"`
	OptionID      string             `bson:"option_id,omitempty" json:"optionId,omitempty"`
	Delta         int64              `bson:"delta,omitempty" json:"delta,omitempty"`
	Counts        map[string]int64   `bson:"counts,omitempty" json:"counts,omitempty"`
	Status        string             `bson:"status,omitempty" json:"status,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"createdAt"`
	NextAttemptAt time.Time          `bson:"next_attempt_at" json:"-"`
	Attempts      int                `bson:"attempts" json:"-"`
	ProcessedAt   *time.Time         `bson:"processed_at,omitempty" json:"-"`
	ClaimedUntil  *time.Time         `bson:"claimed_until,omitempty" json:"-"`
}

// VoteSnapshot is the real-time view handed to connected clients.
type VoteSnapshot struct {
	OptionID string `json:"optionId"`
	Count    int64  `json:"count"`
}

// PollView is a poll plus its current live counts, what the API returns.
type PollView struct {
	Poll
	TotalVotes     int64            `json:"totalVotes"`
	LiveCounts     map[string]int64 `json:"liveCounts,omitempty"`
	CanViewResults bool             `json:"canViewResults"`
}
