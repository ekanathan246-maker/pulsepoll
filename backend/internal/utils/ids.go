package utils

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const optAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomString(alphabet string, n int) (string, error) {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		sb.WriteByte(alphabet[idx.Int64()])
	}
	return sb.String(), nil
}

// NewSlug returns a short URL-safe identifier for a poll.
func NewSlug() (string, error) {
	return randomString(slugAlphabet, 8)
}

// NewOptionID returns a compact identifier for a poll option.
func NewOptionID() (string, error) {
	return randomString(optAlphabet, 4)
}

// NewVoterID returns an anonymous identifier for a viewer's device.
func NewVoterID() (string, error) {
	return randomString(optAlphabet, 16)
}

// NewEventID returns a high-entropy idempotency key for outbox delivery.
func NewEventID() (string, error) {
	return randomString(optAlphabet, 24)
}
