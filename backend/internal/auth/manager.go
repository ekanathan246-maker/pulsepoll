package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"pulsepoll/backend/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUnauthenticated = errors.New("authentication required")

const sessionLifetime = 7 * 24 * time.Hour

type Store interface {
	Insert(context.Context, models.Session) error
	FindByTokenHash(context.Context, string) (models.Session, error)
	DeleteByTokenHash(context.Context, string) error
}

type Manager struct {
	store Store
	now   func() time.Time
}

type Credentials struct {
	Token     string
	CSRFToken string
}

func NewManager(store Store, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{store: store, now: now}
}

func (m *Manager) Create(ctx context.Context, userID primitive.ObjectID) (Credentials, error) {
	token, err := randomToken()
	if err != nil {
		return Credentials{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Credentials{}, err
	}
	now := m.now().UTC()
	session := models.Session{
		ID:        primitive.NewObjectID(),
		TokenHash: tokenHash(token),
		CSRFHash:  tokenHash(csrf),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionLifetime),
	}
	if err := m.store.Insert(ctx, session); err != nil {
		return Credentials{}, err
	}
	return Credentials{Token: token, CSRFToken: csrf}, nil
}

func (m *Manager) Authenticate(ctx context.Context, token string) (models.Session, error) {
	if token == "" {
		return models.Session{}, ErrUnauthenticated
	}
	session, err := m.store.FindByTokenHash(ctx, tokenHash(token))
	if err != nil || !session.ExpiresAt.After(m.now().UTC()) {
		return models.Session{}, ErrUnauthenticated
	}
	return session, nil
}

func (m *Manager) ValidCSRF(session models.Session, token string) bool {
	if token == "" {
		return false
	}
	want := []byte(session.CSRFHash)
	got := []byte(tokenHash(token))
	return len(want) == len(got) && subtle.ConstantTimeCompare(want, got) == 1
}

func (m *Manager) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return m.store.DeleteByTokenHash(ctx, tokenHash(token))
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
