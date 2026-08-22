package http

import (
	"sync"

	"github.com/memomarium/memomarium/internal/session"
)

// AdminManager tracks Local Admin session tokens in memory.
//
// Design note: admin tokens are intentionally kept in an in-memory map guarded
// by a mutex rather than persisted. A Local Admin is a trusted operator on the
// loopback interface; tokens are short-lived for the process lifetime and are
// never written to disk, which keeps the admin boundary simple and avoids
// persisting privileged credentials. This is acceptable for the local-party
// threat model where the admin is the person running the server.
type AdminManager struct {
	mu     sync.Mutex
	tokens map[string]bool
}

// NewAdminManager returns an empty AdminManager.
func NewAdminManager() *AdminManager {
	return &AdminManager{tokens: map[string]bool{}}
}

// Create issues a new admin token.
func (a *AdminManager) Create() (string, error) {
	token, err := session.NewToken()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.tokens[token] = true
	a.mu.Unlock()
	return token, nil
}

// Validate reports whether token is a currently valid admin token.
func (a *AdminManager) Validate(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tokens[token]
}

// Revoke invalidates an admin token.
func (a *AdminManager) Revoke(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.tokens, token)
}
