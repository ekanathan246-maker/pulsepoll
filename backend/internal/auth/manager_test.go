package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pulsepoll/backend/internal/auth"
	"pulsepoll/backend/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memorySessions struct{ session models.Session }

func (m *memorySessions) Insert(_ context.Context, session models.Session) error {
	m.session = session
	return nil
}

func (m *memorySessions) FindByTokenHash(_ context.Context, hash string) (models.Session, error) {
	if m.session.TokenHash != hash {
		return models.Session{}, errors.New("not found")
	}
	return m.session, nil
}

func (m *memorySessions) DeleteByTokenHash(_ context.Context, hash string) error {
	if m.session.TokenHash == hash {
		m.session = models.Session{}
	}
	return nil
}

func TestManagerCreatesOpaqueSessionAndAuthenticatesIt(t *testing.T) {
	store := &memorySessions{}
	now := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	manager := auth.NewManager(store, func() time.Time { return now })
	userID := primitive.NewObjectID()

	credentials, err := manager.Create(context.Background(), userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if credentials.Token == "" || credentials.CSRFToken == "" {
		t.Fatal("Create() returned empty browser credentials")
	}
	if store.session.TokenHash == credentials.Token || store.session.CSRFHash == credentials.CSRFToken {
		t.Fatal("store received a raw browser credential")
	}

	session, err := manager.Authenticate(context.Background(), credentials.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if session.UserID != userID {
		t.Fatalf("Authenticate() user = %s, want %s", session.UserID, userID)
	}
	if !manager.ValidCSRF(session, credentials.CSRFToken) {
		t.Fatal("ValidCSRF() rejected the issued token")
	}
	if manager.ValidCSRF(session, "wrong-token") {
		t.Fatal("ValidCSRF() accepted an invalid token")
	}
}

func TestManagerRejectsExpiredSession(t *testing.T) {
	store := &memorySessions{}
	now := time.Now().UTC()
	manager := auth.NewManager(store, func() time.Time { return now })
	credentials, err := manager.Create(context.Background(), primitive.NewObjectID())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store.session.ExpiresAt = now.Add(-time.Second)

	if _, err := manager.Authenticate(context.Background(), credentials.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("Authenticate() error = %v, want ErrUnauthenticated", err)
	}
}
