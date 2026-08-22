// Package session provides player session creation and authentication.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/repository"
)

// ErrInvalidToken is returned when a token does not match a valid, non-revoked
// session.
var ErrInvalidToken = errors.New("invalid token")

// NewToken returns a cryptographically random session token (32 random bytes
// base64url encoded). The token is never equal to a player ID or session ID.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the SHA-256 hex digest of a token. Only the hash is ever
// stored or compared; the raw token is never persisted.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Service manages player sessions on top of the repository.
type Service struct {
	repo repository.Repository
}

// NewService returns a session Service backed by repo.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// CreateSession creates a session for the given player and returns the raw
// token. The raw token is returned only here and is never stored in the
// database; only its hash is persisted.
func (s *Service) CreateSession(ctx context.Context, playerID string) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	sess := player.PlayerSession{
		ID:        uuid.NewString(),
		PlayerID:  playerID,
		TokenHash: HashToken(token),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}

// Authenticate validates a raw token, rejects revoked sessions, updates
// last_seen_at, and returns the matching session.
func (s *Service) Authenticate(ctx context.Context, token string) (*player.PlayerSession, error) {
	sess, err := s.repo.GetSessionByTokenHash(ctx, HashToken(token))
	if err != nil {
		return nil, ErrInvalidToken
	}
	if sess.RevokedAt != nil {
		return nil, ErrInvalidToken
	}
	now := time.Now().UTC()
	sess.LastSeenAt = &now
	if err := s.repo.UpdateSession(ctx, sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// Revoke revokes a single session by ID.
func (s *Service) Revoke(ctx context.Context, sessionID string) error {
	sess, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sess.RevokedAt = &now
	return s.repo.UpdateSession(ctx, sess)
}

// RevokeAllForPlayer revokes every session belonging to the given player.
func (s *Service) RevokeAllForPlayer(ctx context.Context, playerID string) error {
	sessions, err := s.repo.ListSessionsByPlayer(ctx, playerID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, sess := range sessions {
		if sess.RevokedAt != nil {
			continue
		}
		sess.RevokedAt = &now
		if err := s.repo.UpdateSession(ctx, sess); err != nil {
			return err
		}
	}
	return nil
}
