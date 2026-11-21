package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	coderwebsocket "github.com/coder/websocket"

	"github.com/memomarium/memomarium/internal/coordinator"
	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/scoring"
	"github.com/memomarium/memomarium/internal/engine"
	"github.com/memomarium/memomarium/internal/media"
	"github.com/memomarium/memomarium/internal/repository/sqlite"
	"github.com/memomarium/memomarium/internal/session"
	storagesqlite "github.com/memomarium/memomarium/internal/storage/sqlite"
	"github.com/memomarium/memomarium/internal/transport/websocket"
)

// newTestServer builds a fully wired HTTP server over a temporary SQLite DB.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storagesqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storagesqlite.Migrate(db, dbPath, filepath.Join(dir, "backups")); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := sqlite.New(db)
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		m := content.Meme{
			ID:               uuid.NewString(),
			OriginalPath:     "/o.png",
			ScreenPath:       "/s.png",
			ThumbnailPath:    "/t.png",
			OriginalFilename: "o.png",
			MimeType:         "image/png",
			SHA256:           uuid.NewString(),
			Enabled:          true,
			Source:           "upload",
			CreatedAt:        time.Now().UTC(),
		}
		if err := repo.CreateMeme(ctx, m); err != nil {
			t.Fatalf("seed meme: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		s := content.Situation{
			ID:        uuid.NewString(),
			Text:      "Situation " + string(rune('A'+i)),
			Enabled:   true,
			Source:    "manual",
			CreatedAt: time.Now().UTC(),
		}
		if err := repo.CreateSituation(ctx, s); err != nil {
			t.Fatalf("seed situation: %v", err)
		}
	}

	sessions := session.NewService(repo)
	hub := websocket.NewHub()
	coord := coordinator.New(repo, engine.New(), sessions, hub)
	srv := &Server{
		Coordinator: coord,
		Sessions:    sessions,
		Media:       &media.Service{UploadsDir: dir, MaxBytes: 20 << 20},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Hub:         hub,
		Admin:       NewAdminManager(),
	}
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any, cookies []*http.Cookie) (*http.Response, []*http.Cookie) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, url, err)
	}
	return res, res.Cookies()
}

func TestHandleJoinRestoresExistingSession(t *testing.T) {
	ts := newTestServer(t)
	client := &http.Client{}

	// Admin bootstrap + room creation.
	adminRes, adminCookies := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/admin/bootstrap", nil, nil)
	defer adminRes.Body.Close()
	if adminRes.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap: %d", adminRes.StatusCode)
	}
	createRes, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms", map[string]string{"name": "Host"}, adminCookies)
	defer createRes.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode room: %v", err)
	}

	// First join: creates Alice.
	joinRes, cookies := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Alice"}, nil)
	defer joinRes.Body.Close()
	if joinRes.StatusCode != http.StatusOK {
		t.Fatalf("join: %d", joinRes.StatusCode)
	}
	var snap map[string]any
	if err := json.NewDecoder(joinRes.Body).Decode(&snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	players := snap["players"].([]any)
	if len(players) != 1 {
		t.Fatalf("expected 1 player after first join, got %d", len(players))
	}

	// Same browser (same session cookie) "joins" again: must restore the
	// existing player, not create a duplicate (spec §29).
	joinRes2, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Bob"}, cookies)
	defer joinRes2.Body.Close()
	if joinRes2.StatusCode != http.StatusOK {
		t.Fatalf("re-join: %d", joinRes2.StatusCode)
	}
	var snap2 map[string]any
	if err := json.NewDecoder(joinRes2.Body).Decode(&snap2); err != nil {
		t.Fatalf("decode snapshot 2: %v", err)
	}
	players2 := snap2["players"].([]any)
	if len(players2) != 1 {
		t.Fatalf("expected 1 player after re-join (restore, not duplicate), got %d", len(players2))
	}
}

func TestKickClosesPlayerWebSocket(t *testing.T) {
	ts := newTestServer(t)
	client := &http.Client{}

	adminRes, adminCookies := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/admin/bootstrap", nil, nil)
	defer adminRes.Body.Close()
	createRes, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms", map[string]string{"name": "Host"}, adminCookies)
	defer createRes.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode room: %v", err)
	}

	// Join a player over HTTP to get their session cookie.
	joinRes, playerCookies := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Alice"}, nil)
	defer joinRes.Body.Close()
	var snap struct {
		Actor struct {
			PlayerID string `json:"playerId"`
		} `json:"actor"`
	}
	if err := json.NewDecoder(joinRes.Body).Decode(&snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snap.Actor.PlayerID == "" {
		t.Fatal("expected actor player id")
	}

	// Open a real WebSocket as that player.
	wsURL := "ws" + ts.URL[len("http"):] + "/api/v1/rooms/" + created.Code + "/ws"
	wsConn, _, err := coderwebsocket.Dial(context.Background(), wsURL, &coderwebsocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": []string{playerCookies[0].String()}},
	})
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer wsConn.Close(coderwebsocket.StatusNormalClosure, "")

	// Receive the initial snapshot.
	if _, _, err := wsConn.Read(context.Background()); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	// Admin kicks the player.
	kickRes, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/commands",
		map[string]any{
			"commandId":        uuid.NewString(),
			"expectedRevision": -1,
			"type":             "KICK_PLAYER",
			"payload":          map[string]string{"playerId": snap.Actor.PlayerID},
		}, adminCookies)
	defer kickRes.Body.Close()
	if kickRes.StatusCode != http.StatusOK {
		t.Fatalf("kick: %d", kickRes.StatusCode)
	}

	// The player's connection must be closed by the server with the
	// session-revoked close code. The close runs asynchronously after the kick
	// command, so wait for it with a per-read deadline.
	var lastErr error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _, lastErr = wsConn.Read(readCtx)
		cancel()
		if lastErr != nil {
			break
		}
	}
	if lastErr == nil {
		t.Fatal("expected the kicked player's websocket to be closed")
	}
	if closeCode := coderwebsocket.CloseStatus(lastErr); closeCode != coderwebsocket.StatusCode(websocket.SessionRevokedCloseCode) {
		t.Fatalf("expected close code %d, got %d (err=%v)", websocket.SessionRevokedCloseCode, closeCode, lastErr)
	}
}

func TestJoinRejectedWhileInGame(t *testing.T) {
	ts := newTestServer(t)
	client := &http.Client{}

	adminRes, adminCookies := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/admin/bootstrap", nil, nil)
	defer adminRes.Body.Close()
	createRes, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms", map[string]string{"name": "Host"}, adminCookies)
	defer createRes.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode room: %v", err)
	}

	// Two players join.
	join1, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Alice"}, nil)
	join1.Body.Close()
	join2, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Bob"}, nil)
	join2.Body.Close()

	// Start the game (admin).
	startRes, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/commands",
		map[string]any{"commandId": uuid.NewString(), "expectedRevision": -1, "type": "START_GAME"}, adminCookies)
	defer startRes.Body.Close()
	if startRes.StatusCode != http.StatusOK {
		t.Fatalf("start game: %d", startRes.StatusCode)
	}

	// A brand-new browser (no session cookie) must be rejected while the game
	// is running.
	join3, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/rooms/"+created.Code+"/join", map[string]string{"name": "Carol"}, nil)
	defer join3.Body.Close()
	if join3.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for join during game, got %d", join3.StatusCode)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(join3.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errBody.Code != "GAME_ALREADY_STARTED" {
		t.Fatalf("expected GAME_ALREADY_STARTED, got %s", errBody.Code)
	}
}

func TestApplySettingsPatch(t *testing.T) {
	t.Run("full patch overrides every field", func(t *testing.T) {
		current := room.DefaultRoomSettings()
		patch := settingsPatch{
			MinPlayers:                   intPtr(3),
			MaxPlayers:                   intPtr(12),
			HandSize:                     intPtr(7),
			PreparationTimeoutSeconds:    intPtr(30),
			RoundSelectionTimeoutSeconds: intPtr(45),
			VotingTimeoutSeconds:         intPtr(60),
			InfiniteGame:                 boolPtr(true),
			SituationSeparator:           strPtr("###"),
			ScoreConfig:                  &scoring.ScoreConfig{PartialGuesser: 9},
		}
		got := applySettingsPatch(current, patch)
		if got.MinPlayers != 3 || got.MaxPlayers != 12 || got.HandSize != 7 {
			t.Fatalf("players not patched: %+v", got)
		}
		if got.PreparationTimeoutSeconds != 30 || got.RoundSelectionTimeoutSeconds != 45 || got.VotingTimeoutSeconds != 60 {
			t.Fatalf("timers not patched: %+v", got)
		}
		if !got.InfiniteGame || got.SituationSeparator != "###" || got.ScoreConfig.PartialGuesser != 9 {
			t.Fatalf("mode/separator/score not patched: %+v", got)
		}
	})

	t.Run("explicit zero timer is applied (disables a timer)", func(t *testing.T) {
		current := room.DefaultRoomSettings()
		current.VotingTimeoutSeconds = 120
		patch := settingsPatch{VotingTimeoutSeconds: intPtr(0)}
		got := applySettingsPatch(current, patch)
		if got.VotingTimeoutSeconds != 0 {
			t.Fatalf("expected timer 0, got %d", got.VotingTimeoutSeconds)
		}
	})

	t.Run("nil fields keep current values", func(t *testing.T) {
		current := room.DefaultRoomSettings()
		current.HandSize = 9
		patch := settingsPatch{InfiniteGame: boolPtr(true)}
		got := applySettingsPatch(current, patch)
		if got.HandSize != 9 || got.MinPlayers != current.MinPlayers {
			t.Fatalf("unset fields must be preserved: %+v", got)
		}
	})
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestIsLocalClient(t *testing.T) {
	t.Run("genuine loopback IPv4", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "127.0.0.1:54321"}
		if !isLocalClient(r) {
			t.Fatal("expected 127.0.0.1 to be local")
		}
	})

	t.Run("genuine loopback IPv6", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "[::1]:54321"}
		if !isLocalClient(r) {
			t.Fatal("expected ::1 to be local")
		}
	})

	t.Run("one of this machine's own addresses", func(t *testing.T) {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			t.Skipf("InterfaceAddrs failed: %v", err)
		}
		checked := 0
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			r := &http.Request{RemoteAddr: net.JoinHostPort(ipnet.IP.String(), "54321")}
			if !isLocalClient(r) {
				t.Fatalf("expected own address %s to be local", ipnet.IP)
			}
			checked++
		}
		if checked == 0 {
			t.Skip("no interface addresses to check")
		}
	})

	t.Run("foreign LAN peer is denied", func(t *testing.T) {
		// TEST-NET-3 (203.0.113.0/24) is reserved for documentation and never
		// assigned to real interfaces, so it can never be one of our addresses.
		r := &http.Request{RemoteAddr: "203.0.113.7:54321"}
		if isLocalClient(r) {
			t.Fatal("expected a foreign LAN peer to be denied")
		}
	})

	t.Run("forwarded header is not trusted", func(t *testing.T) {
		// A non-local peer spoofing X-Forwarded-For must NOT be treated as
		// local: isLocalClient only inspects the raw TCP peer (RemoteAddr).
		r := &http.Request{
			RemoteAddr: "203.0.113.7:54321",
			Header:     http.Header{"X-Forwarded-For": []string{"127.0.0.1"}},
		}
		if isLocalClient(r) {
			t.Fatal("X-Forwarded-For must not make a foreign peer local")
		}
	})

	t.Run("malformed remote addr", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "not-an-addr"}
		if isLocalClient(r) {
			t.Fatal("expected malformed RemoteAddr to be denied")
		}
	})
}
