package session

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/repository/sqlite"
	storagesqlite "github.com/memomarium/memomarium/internal/storage/sqlite"
)

func newTestRepo(t *testing.T) *sqlite.Repo {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storagesqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, storagesqlite.Migrate(db, dbPath, filepath.Join(dir, "backups")))
	return sqlite.New(db)
}

func newPlayer(t *testing.T, repo *sqlite.Repo) player.Player {
	t.Helper()
	ctx := context.Background()
	r := room.Room{ID: uuid.NewString(), Code: "ABC123", Revision: 0, State: room.StateLobby}
	require.NoError(t, repo.CreateRoom(ctx, r))
	p := player.Player{ID: uuid.NewString(), RoomID: r.ID, Name: "Alice", Role: player.RolePlayer, Connected: true}
	require.NoError(t, repo.CreatePlayer(ctx, p))
	return p
}

func TestNewTokenUniqueAndNotEqualIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := NewToken()
		require.NoError(t, err)
		require.NotEmpty(t, tok)
		require.False(t, seen[tok], "token must be unique")
		seen[tok] = true
		require.NotEqual(t, "player-1", tok)
		require.NotEqual(t, "session-1", tok)
	}
}

func TestHashTokenNotReversible(t *testing.T) {
	tok, err := NewToken()
	require.NoError(t, err)
	h := HashToken(tok)
	require.NotEqual(t, tok, h)
	require.Len(t, h, 64) // SHA-256 hex
	// Same token hashes identically.
	require.Equal(t, h, HashToken(tok))
	// Different token hashes differently.
	other, err := NewToken()
	require.NoError(t, err)
	require.NotEqual(t, h, HashToken(other))
}

func TestCreateAuthenticateRevoke(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	svc := NewService(repo)
	p := newPlayer(t, repo)

	token, err := svc.CreateSession(ctx, p.ID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// The raw token must not be stored; only its hash.
	stored, err := repo.GetSessionByTokenHash(ctx, HashToken(token))
	require.NoError(t, err)
	require.NotEqual(t, token, stored.TokenHash)
	require.Equal(t, HashToken(token), stored.TokenHash)

	sess, err := svc.Authenticate(ctx, token)
	require.NoError(t, err)
	require.Equal(t, p.ID, sess.PlayerID)
	require.Nil(t, sess.RevokedAt)
	require.NotNil(t, sess.LastSeenAt)

	require.NoError(t, svc.Revoke(ctx, sess.ID))

	// Revoked session must be rejected.
	_, err = svc.Authenticate(ctx, token)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestRevokeAllForPlayer(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	svc := NewService(repo)
	p := newPlayer(t, repo)

	t1, err := svc.CreateSession(ctx, p.ID)
	require.NoError(t, err)
	t2, err := svc.CreateSession(ctx, p.ID)
	require.NoError(t, err)

	require.NoError(t, svc.RevokeAllForPlayer(ctx, p.ID))

	_, err = svc.Authenticate(ctx, t1)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = svc.Authenticate(ctx, t2)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestAuthenticateInvalidToken(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	svc := NewService(repo)
	_, err := svc.Authenticate(ctx, "not-a-real-token")
	require.ErrorIs(t, err, ErrInvalidToken)
}
