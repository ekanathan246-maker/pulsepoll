package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestEnsureIndexesMigratesLegacyVoteIndex(t *testing.T) {
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

	if err := EnsureIndexes(ctx, uri, dbName); err != nil {
		t.Fatalf("EnsureIndexes failed to migrate the legacy index: %v", err)
	}

	cursor, err := votes.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	foundUnique := false
	for cursor.Next(ctx) {
		var index struct {
			Name   string `bson:"name"`
			Unique bool   `bson:"unique"`
		}
		if err := cursor.Decode(&index); err != nil {
			t.Fatal(err)
		}
		if index.Name == voteIdentityIndexName {
			foundUnique = index.Unique
		}
	}
	if err := cursor.Err(); err != nil {
		t.Fatal(err)
	}
	if !foundUnique {
		t.Fatalf("%s was not migrated to a unique index", voteIdentityIndexName)
	}
}
