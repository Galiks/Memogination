package player

import "time"

// PlayerSession is an authenticated session for a player.
type PlayerSession struct {
	ID         string
	PlayerID   string
	TokenHash  string
	CreatedAt  time.Time
	LastSeenAt *time.Time
	RevokedAt  *time.Time
}
