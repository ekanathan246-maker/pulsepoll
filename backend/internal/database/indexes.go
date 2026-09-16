package database

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

	polls := db.Collection("polls")
	if _, err := polls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	if _, err := polls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "created_by", Value: 1}},
	}); err != nil {
		return err
	}

	votes := db.Collection("votes")
	if _, err := votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_id", Value: 1}},
	}); err != nil {
		return err
	}
	return nil
}
