package database

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Databases bundles the connectivity the rest of the app depends on.
type Databases struct {
	Mongo *mongo.Database
	Redis *redis.Client
}

func (d *Databases) Close(ctx context.Context) error {
	mongoErr := d.Mongo.Client().Disconnect(ctx)
	redisErr := d.Redis.Close()
	if mongoErr != nil {
		return mongoErr
	}
	return redisErr
}

// Connect establishes and pings both Mongo and Redis then returns a Databases value.
func Connect(ctx context.Context, mongoURI, mongoDB, redisAddr, redisPass, redisURL string) (*Databases, error) {
	mc, err := connectMongo(ctx, mongoURI)
	if err != nil {
		return nil, err
	}
	rc, err := connectRedis(redisAddr, redisPass, redisURL)
	if err != nil {
		return nil, err
	}
	if err := rc.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Databases{Mongo: mc.Database(mongoDB), Redis: rc}, nil
}

func connectMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}
	log.Println("connected to MongoDB")
	return client, nil
}

func connectRedis(addr, password, rawURL string) (*redis.Client, error) {
	if rawURL != "" {
		options, err := redis.ParseURL(rawURL)
		if err != nil {
			return nil, err
		}
		return redis.NewClient(options), nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	}), nil
}
