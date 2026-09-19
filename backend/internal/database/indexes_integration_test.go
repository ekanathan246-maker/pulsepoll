package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestEnsureIndexesBackfillsVoteClaimsWithoutDeletingLegacyDuplicates(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("pulsepoll_index_migration_%d", time.Now().UnixNano())

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
	defer client.Database(dbName).Drop(ctx)

	votes := client.Database(dbName).Collection("votes")
	if _, err := votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_id", Value: 1}},
		Options: options.Index().SetName(voteIdentityIndexName),
	}); err != nil {
		t.Fatal(err)
	}
	pollID := primitive.NewObjectID()
	if _, err := votes.InsertMany(ctx, []any{
		bson.M{"poll_id": pollID, "voter_id": "same-browser", "option_id": "a"},
		bson.M{"poll_id": pollID, "voter_id": "same-browser", "option_id": "b"},
		bson.M{"poll_id": pollID, "voter_id": "another-browser", "option_id": "a"},
		bson.M{"poll_id": pollID, "option_id": "legacy-missing-voter"},
		bson.M{"poll_id": pollID, "voter_id": nil, "option_id": "legacy-null-voter"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := EnsureIndexes(ctx, uri, dbName); err != nil {
		t.Fatalf("EnsureIndexes failed to backfill vote claims: %v", err)
	}
	if err := EnsureIndexes(ctx, uri, dbName); err != nil {
		t.Fatalf("EnsureIndexes was not idempotent: %v", err)
	}

	if count, err := votes.CountDocuments(ctx, bson.M{}); err != nil || count != 5 {
		t.Fatalf("historical votes changed: count=%d err=%v", count, err)
	}
	voteClaims := client.Database(dbName).Collection("vote_claims")
	if count, err := voteClaims.CountDocuments(ctx, bson.M{}); err != nil || count != 2 {
		t.Fatalf("expected one claim per poll/voter pair: count=%d err=%v", count, err)
	}
	if _, err := voteClaims.InsertOne(ctx, bson.M{"poll_id": pollID, "voter_id": "same-browser"}); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("vote claim index did not reject a duplicate: %v", err)
	}
}
