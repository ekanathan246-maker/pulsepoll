package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const voteIdentityIndexName = "poll_id_1_voter_id_1"

// EnsureIndexes sets up the indexes we rely on for uniqueness and query speed.
func EnsureIndexes(ctx context.Context, mongoURI, dbName string) error {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	users := db.Collection("users")
	if _, err := users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	sessions := db.Collection("sessions")
	if _, err := sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "token_hash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}); err != nil {
		return err
	}

	polls := db.Collection("polls")
	if _, err := polls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"slug": bson.M{"$type": "string"}}),
	}); err != nil {
		return err
	}
	if _, err := polls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_by", Value: 1}},
	}); err != nil {
		return err
	}

	votes := db.Collection("votes")
	if err := ensureVoteLookupIndex(ctx, votes); err != nil {
		return err
	}
	if err := ensureVoteClaims(ctx, votes, db.Collection("vote_claims")); err != nil {
		return err
	}

	outbox := db.Collection("outbox")
	if _, err := outbox.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "processed_at", Value: 1}, {Key: "next_attempt_at", Value: 1}, {Key: "claimed_until", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "aggregate_id", Value: 1}, {Key: "processed_at", Value: 1}, {Key: "version", Value: 1}},
		},
	}); err != nil {
		return err
	}
	return nil
}

// ensureVoteLookupIndex accepts both the legacy non-unique index and the unique
// index used by a short-lived release. Duplicate-vote enforcement now lives in
// vote_claims, so production can start without deleting historical ballots.
func ensureVoteLookupIndex(ctx context.Context, votes *mongo.Collection) error {
	cursor, err := votes.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("list vote indexes: %w", err)
	}
	found := false
	for cursor.Next(ctx) {
		var index struct {
			Name string `bson:"name"`
		}
		if err := cursor.Decode(&index); err != nil {
			return fmt.Errorf("decode vote index: %w", err)
		}
		if index.Name == voteIdentityIndexName {
			found = true
			break
		}
	}
	cursorErr := cursor.Err()
	cursor.Close(ctx)
	if cursorErr != nil {
		return fmt.Errorf("scan vote indexes: %w", cursorErr)
	}

	if found {
		return nil
	}

	if _, err := votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_id", Value: 1}},
		Options: options.Index().SetName(voteIdentityIndexName),
	}); err != nil {
		return fmt.Errorf("create vote lookup index: %w", err)
	}
	return nil
}

// ensureVoteClaims builds a unique guard from every historical ballot. The
// $group stage deliberately collapses legacy duplicates into one claim while
// leaving the append-only votes ledger and poll totals unchanged.
func ensureVoteClaims(ctx context.Context, votes, voteClaims *mongo.Collection) error {
	if _, err := voteClaims.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_id", Value: 1}},
		Options: options.Index().
			SetName(voteIdentityIndexName).
			SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create unique vote claim index: %w", err)
	}

	cursor, err := votes.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "poll_id", Value: "$poll_id"}, {Key: "voter_id", Value: "$voter_id"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "poll_id", Value: "$_id.poll_id"},
			{Key: "voter_id", Value: "$_id.voter_id"},
		}}},
		{{Key: "$merge", Value: bson.D{
			{Key: "into", Value: voteClaims.Name()},
			{Key: "on", Value: bson.A{"poll_id", "voter_id"}},
			{Key: "whenMatched", Value: "keepExisting"},
			{Key: "whenNotMatched", Value: "insert"},
		}}},
	})
	if err != nil {
		return fmt.Errorf("backfill vote claims: %w", err)
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &[]bson.M{}); err != nil {
		return fmt.Errorf("finish vote claim backfill: %w", err)
	}
	return nil
}
