package auth

import (
	"context"

	"pulsepoll/backend/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoStore struct{ sessions *mongo.Collection }

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{sessions: db.Collection("sessions")}
}

func (s *MongoStore) Insert(ctx context.Context, session models.Session) error {
	_, err := s.sessions.InsertOne(ctx, session)
	return err
}

func (s *MongoStore) FindByTokenHash(ctx context.Context, hash string) (models.Session, error) {
	var session models.Session
	err := s.sessions.FindOne(ctx, bson.M{"token_hash": hash}).Decode(&session)
	return session, err
}

func (s *MongoStore) DeleteByTokenHash(ctx context.Context, hash string) error {
	_, err := s.sessions.DeleteOne(ctx, bson.M{"token_hash": hash})
	return err
}
