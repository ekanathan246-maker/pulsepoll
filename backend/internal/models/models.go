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
}

// Vote records a single ballot, kept as an append-only ledger in Mongo.
type Vote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PollID    string             `bson:"poll_id" json:"poll_id"`
	OptionID  string             `bson:"option_id" json:"option_id"`
	VoterID   string             `bson:"voter_id" json:"voter_id"`
	UserAgent string             `bson:"user_agent,omitempty" json:"-"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// VoteSnapshot is the real-time view handed to connected clients.
type VoteSnapshot struct {
	OptionID string `json:"optionId"`
	Count    int64  `json:"count"`
}

// PollView is a poll plus its current live counts, what the API returns.
type PollView struct {
	Poll
	TotalVotes int64            `json:"totalVotes"`
	LiveCounts map[string]int64 `json:"liveCounts,omitempty"`
}
