package database

import (
	"context"
	"fmt"
	"log"

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
	if err := ensureUniqueVoteIndex(ctx, votes); err != nil {
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

// ensureUniqueVoteIndex upgrades the non-unique vote identity index created by
// early PulsePoll releases. MongoDB generates the same name for both index
// definitions, so blindly calling CreateOne with unique=true crashes every
// subsequent startup with IndexOptionsConflict.
func ensureUniqueVoteIndex(ctx context.Context, votes *mongo.Collection) error {
	cursor, err := votes.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("list vote indexes: %w", err)
	}
	needsReplacement := false
	for cursor.Next(ctx) {
		var index struct {
			Name   string `bson:"name"`
			Unique bool   `bson:"unique"`
		}
		if err := cursor.Decode(&index); err != nil {
			return fmt.Errorf("decode vote index: %w", err)
		}
		if !voteIndexNeedsReplacement(index.Name, index.Unique) {
			continue
		}

		needsReplacement = true
		break
	}
	cursorErr := cursor.Err()
	cursor.Close(ctx)
	if cursorErr != nil {
		return fmt.Errorf("scan vote indexes: %w", cursorErr)
	}

	if needsReplacement {
		duplicateCursor, err := votes.Aggregate(ctx, mongo.Pipeline{
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "poll_id", Value: "$poll_id"}, {Key: "voter_id", Value: "$voter_id"}}},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}}},
			{{Key: "$match", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
			{{Key: "$limit", Value: 1}},
		})
		if err != nil {
			return fmt.Errorf("check duplicate votes before index migration: %w", err)
		}
		hasDuplicates := duplicateCursor.Next(ctx)
		duplicateErr := duplicateCursor.Err()
		duplicateCursor.Close(ctx)
		if duplicateErr != nil {
			return fmt.Errorf("scan duplicate votes before index migration: %w", duplicateErr)
		}
		if hasDuplicates {
			return fmt.Errorf("cannot make %s unique: duplicate poll/voter records exist", voteIdentityIndexName)
		}

		if _, err := votes.Indexes().DropOne(ctx, voteIdentityIndexName); err != nil {
			return fmt.Errorf("drop legacy vote index %s: %w", voteIdentityIndexName, err)
		}
		log.Printf("migrated legacy vote index %s to unique", voteIdentityIndexName)
	}

	if _, err := votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_id", Value: 1}},
		Options: options.Index().
			SetName(voteIdentityIndexName).
			SetUnique(true),
	}); err != nil {
		return fmt.Errorf("create unique vote identity index: %w", err)
	}
	return nil
}

func voteIndexNeedsReplacement(name string, unique bool) bool {
	return name == voteIdentityIndexName && !unique
}
