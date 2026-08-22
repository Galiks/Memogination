// Package http implements the HTTP/REST transport for Memomarium.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	coderwebsocket "github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/memomarium/memomarium/internal/app"
	"github.com/memomarium/memomarium/internal/config"
	"github.com/memomarium/memomarium/internal/coordinator"
	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/engine"
	"github.com/memomarium/memomarium/internal/media"
	"github.com/memomarium/memomarium/internal/projection"
	"github.com/memomarium/memomarium/internal/session"
	"github.com/memomarium/memomarium/internal/transport/websocket"
)

const (
	sessionCookie = "memomarium_session"
	adminCookie   = "memomarium_admin"
)

// Server is the HTTP transport for the game.
type Server struct {
	Coordinator *coordinator.Coordinator
	Sessions    *session.Service
	Media       *media.Service
	Config      config.Config
	Logger      *slog.Logger
	Hub         *websocket.Hub
	// App provides Health() for the /health endpoint.
	App *app.App
	// Admin manages Local Admin session tokens.
	Admin *AdminManager
}

// New returns a Server wired to the given dependencies.
func New(coord *coordinator.Coordinator, sessions *session.Service, med *media.Service,
	cfg config.Config, logger *slog.Logger, hub *websocket.Hub, application *app.App) *Server {
	return &Server{
		Coordinator: coord,
		Sessions:    sessions,
		Media:       med,
		Config:      cfg,
		Logger:      logger,
		Hub:         hub,
		App:         application,
		Admin:       NewAdminManager(),
	}
}

// Routes builds the chi router for the server.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/network/addresses", s.handleNetworkAddresses)
		r.Post("/admin/bootstrap", s.handleAdminBootstrap)

		r.Post("/rooms", s.handleCreateRoom)
		r.Route("/rooms/{code}", func(r chi.Router) {
			r.Post("/join", s.handleJoin)
			r.Post("/reconnect", s.handleReconnect)
			r.Post("/commands", s.handleCommands)
			r.Get("/state", s.handleState)
			r.Get("/settings", s.handleGetSettings)
			r.Put("/settings", s.handleUpdateSettings)
			r.Get("/ws", s.handleWS)
		})

		r.Get("/memes", s.handleListMemes)
		r.Post("/memes", s.handleCreateMeme)
		r.Delete("/memes/{id}", s.handleDeleteMeme)
		r.Patch("/memes/{id}", s.handleUpdateMeme)

		r.Get("/situations", s.handleListSituations)
		r.Post("/situations", s.handleCreateSituation)
		r.Post("/situations/bulk", s.handleBulkSituations)
		r.Delete("/situations/{id}", s.handleDeleteSituation)
		r.Patch("/situations/{id}", s.handleUpdateSituation)
	})

	// Serve uploaded media.
	r.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir(s.Config.UploadsDir))))

	// Static frontend with SPA fallback for everything else.
	r.NotFound(s.staticHandler().ServeHTTP)

	return r
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// statusForCode maps an engine error code to an HTTP status.
func statusForCode(code string) int {
	switch code {
	case "STATE_CHANGED":
		return http.StatusConflict
	case "NOT_ALLOWED":
		return http.StatusForbidden
	case "ROOM_NOT_FOUND":
		return http.StatusNotFound
	case "INTERNAL":
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// writeError writes an engine error in the standard error contract.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	code := engine.Code(err)
	if code == "" {
		code = "INTERNAL"
	}
	message := err.Error()
	if code == "INTERNAL" {
		// Do not leak internal identifiers to clients; log the real error
		// server-side instead.
		s.Logger.Error("internal error", "error", err)
		message = "internal error"
	}
	body := map[string]any{
		"code":    code,
		"message": message,
		"details": map[string]any{},
	}
	if code == "STATE_CHANGED" {
		if rev, ok := coordinator.StateChangedRevision(err); ok {
			body["currentRevision"] = rev
		}
	}
	writeJSON(w, statusForCode(code), body)
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) isAdmin(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return s.Admin.Validate(c.Value)
}

func (s *Server) sessionCookie(r *http.Request) (string, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return "", engine.ErrInvalidSession
	}
	return c.Value, nil
}

func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "8080"
	}
	return port
}

// --- health & network ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Health(r.Context()))
}

func (s *Server) handleNetworkAddresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"addresses": DetectLANAddresses(portFromAddr(s.Config.HTTPAddr)),
	})
}

// --- admin ---

func (s *Server) handleAdminBootstrap(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	token, err := s.Admin.Create()
	if err != nil {
		s.writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"isAdmin": true})
}

// --- rooms ---

// handleCreateRoom creates a new room and returns it with its join code. This
// endpoint is a minimal addition beyond the original spec: without a way to
// create a room, the join endpoint would be unusable. The coordinator's
// CreateRoom method is exposed here.
func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidName)
		return
	}
	room, err := s.Coordinator.CreateRoom(r.Context(), body.Name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidName)
		return
	}
	token, snap, err := s.Coordinator.JoinRoom(r.Context(), code, body.Name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleReconnect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	token, err := s.sessionCookie(r)
	if err != nil {
		s.writeError(w, engine.ErrInvalidSession)
		return
	}
	snap, err := s.Coordinator.Reconnect(r.Context(), code, token)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var body struct {
		CommandID        string         `json:"commandId"`
		ExpectedRevision int            `json:"expectedRevision"`
		Type             string         `json:"type"`
		Payload          map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidCommand)
		return
	}

	isAdmin := s.isAdmin(r)
	var playerID string
	if !isAdmin {
		token, err := s.sessionCookie(r)
		if err != nil {
			s.writeError(w, engine.ErrNotAllowed)
			return
		}
		sess, err := s.Sessions.Authenticate(r.Context(), token)
		if err != nil {
			s.writeError(w, engine.ErrInvalidSession)
			return
		}
		playerID = sess.PlayerID
	}

	cmd := engine.Command{
		CommandID: body.CommandID,
		Type:      body.Type,
		Payload:   body.Payload,
		Now:       time.Now().UTC(),
	}
	events, snap, err := s.Coordinator.HandleCommand(r.Context(), code, cmd, body.ExpectedRevision, playerID, isAdmin)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "snapshot": snap})
}

// snapshotFor builds the appropriate projection for the requester.
func (s *Server) snapshotFor(r *http.Request, code string) (*projection.GameSnapshot, error) {
	room, err := s.Coordinator.GetRoomByCode(r.Context(), code)
	if err != nil {
		return nil, err
	}
	agg, err := s.Coordinator.LoadAggregate(r.Context(), room.ID)
	if err != nil {
		return nil, err
	}
	if s.isAdmin(r) {
		return projection.HostSnapshot(agg, ""), nil
	}
	if r.URL.Query().Get("screen") == "1" {
		return projection.ScreenSnapshot(agg), nil
	}
	token, err := s.sessionCookie(r)
	if err != nil {
		return nil, engine.ErrInvalidSession
	}
	sess, err := s.Sessions.Authenticate(r.Context(), token)
	if err != nil {
		return nil, engine.ErrInvalidSession
	}
	// A player with a valid session in another room must not read this room's
	// state.
	if p := agg.PlayerByID(sess.PlayerID); p == nil || p.RoomID != room.ID {
		return nil, engine.ErrPlayerNotFound
	}
	return projection.PlayerSnapshot(agg, sess.PlayerID, false), nil
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	snap, err := s.snapshotFor(r, code)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	settings, err := s.Coordinator.GetSettings(r.Context(), code)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	var settings struct {
		MinPlayers                   int  `json:"minPlayers"`
		MaxPlayers                   int  `json:"maxPlayers"`
		HandSize                     int  `json:"handSize"`
		PreparationTimeoutSeconds    int  `json:"preparationTimeoutSeconds"`
		RoundSelectionTimeoutSeconds int  `json:"roundSelectionTimeoutSeconds"`
		VotingTimeoutSeconds         int  `json:"votingTimeoutSeconds"`
		InfiniteGame                 bool `json:"infiniteGame"`
	}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		s.writeError(w, engine.ErrInvalidSettings)
		return
	}
	rs := roomSettingsFromDTO(settings)
	if err := s.Coordinator.UpdateSettings(r.Context(), code, rs, true); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rs)
}

// --- memes ---

// memeDTO is the public meme representation. originalPath is only populated
// for admins; it is omitted from the public catalog so players cannot map meme
// IDs to original file paths during ROUND_SELECTION.
type memeDTO struct {
	ID               string    `json:"id"`
	ScreenPath       string    `json:"screenPath"`
	ThumbnailPath    string    `json:"thumbnailPath"`
	OriginalFilename string    `json:"originalFilename"`
	MimeType         string    `json:"mimeType"`
	Enabled          bool      `json:"enabled"`
	Source           string    `json:"source"`
	CreatedAt        time.Time `json:"createdAt"`
	OriginalPath     string    `json:"originalPath,omitempty"`
}

func (s *Server) handleListMemes(w http.ResponseWriter, r *http.Request) {
	memes, err := s.Coordinator.ListMemes(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	isAdmin := s.isAdmin(r)
	out := make([]memeDTO, 0, len(memes))
	for _, m := range memes {
		dto := memeDTO{
			ID:               m.ID,
			ScreenPath:       m.ScreenPath,
			ThumbnailPath:    m.ThumbnailPath,
			OriginalFilename: m.OriginalFilename,
			MimeType:         m.MimeType,
			Enabled:          m.Enabled,
			Source:           m.Source,
			CreatedAt:        m.CreatedAt,
		}
		if isAdmin {
			dto.OriginalPath = m.OriginalPath
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateMeme(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(s.Config.MaxUploadBytes); err != nil {
		s.writeError(w, engine.ErrInvalidMeme)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, engine.ErrInvalidMeme)
		return
	}
	defer file.Close()

	// Reject oversized uploads before buffering the whole file.
	if header.Size > s.Config.MaxUploadBytes {
		s.writeError(w, engine.ErrFileTooLarge)
		return
	}

	// Cap the initial capacity at MaxUploadBytes so an attacker-controlled
	// header.Size cannot force a huge preallocation.
	initialCap := header.Size
	if initialCap > s.Config.MaxUploadBytes {
		initialCap = s.Config.MaxUploadBytes
	}
	data := make([]byte, 0, initialCap)
	buf := make([]byte, 64*1024)
	for {
		n, rerr := file.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if rerr != nil {
			if errors.Is(rerr, context.Canceled) {
				s.writeError(w, engine.ErrInvalidMeme)
				return
			}
			break
		}
	}

	res, err := s.Media.ProcessUpload(r.Context(), header.Filename, data)
	if err != nil {
		if errors.Is(err, media.ErrFileTooLarge) {
			s.writeError(w, engine.ErrFileTooLarge)
			return
		}
		s.writeError(w, err)
		return
	}

	// Avoid orphaned files on duplicate upload: if a meme with the same
	// SHA-256 already exists, remove the just-written files and report the
	// duplicate instead of persisting a second copy.
	if existing, derr := s.Coordinator.GetMemeBySHA256(r.Context(), res.SHA256); derr == nil && existing.ID != "" {
		s.Media.RemoveResult(res)
		s.writeError(w, engine.ErrDuplicateMeme)
		return
	}

	m := content.Meme{
		OriginalPath:     res.OriginalPath,
		ScreenPath:       res.ScreenPath,
		ThumbnailPath:    res.ThumbnailPath,
		OriginalFilename: header.Filename,
		MimeType:         res.MimeType,
		SHA256:           res.SHA256,
		Enabled:          true,
		Source:           "upload",
	}
	if err := s.Coordinator.AddMeme(r.Context(), m, true); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleDeleteMeme(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.Coordinator.DeleteMeme(r.Context(), id, true); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleUpdateMeme(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidMeme)
		return
	}
	memes, err := s.Coordinator.ListMemes(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	for _, m := range memes {
		if m.ID == id {
			if body.Enabled != nil {
				m.Enabled = *body.Enabled
			}
			if err := s.Coordinator.UpdateMeme(r.Context(), m, true); err != nil {
				s.writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, m)
			return
		}
	}
	s.writeError(w, engine.ErrInvalidMeme)
}

// --- situations ---

func (s *Server) handleListSituations(w http.ResponseWriter, r *http.Request) {
	situations, err := s.Coordinator.ListSituations(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, situations)
}

func (s *Server) handleCreateSituation(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	var body struct {
		Text    string `json:"text"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidCommand)
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	sit := content.Situation{Text: body.Text, Enabled: enabled, Source: "manual"}
	if err := s.Coordinator.AddSituation(r.Context(), sit, true); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sit)
}

func (s *Server) handleBulkSituations(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	var body struct {
		Text      string `json:"text"`
		Delimiter string `json:"delimiter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidCommand)
		return
	}
	if body.Delimiter == "" {
		body.Delimiter = "*"
	}
	result, err := s.Coordinator.BulkAddSituations(r.Context(), body.Text, body.Delimiter, true)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleDeleteSituation(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.Coordinator.DeleteSituation(r.Context(), id, true); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleUpdateSituation(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		s.writeError(w, engine.ErrNotAllowed)
		return
	}
	id := chi.URLParam(r, "id")
	var body struct {
		Text    string `json:"text"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, engine.ErrInvalidCommand)
		return
	}
	situations, err := s.Coordinator.ListSituations(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	for _, sit := range situations {
		if sit.ID == id {
			if body.Text != "" {
				sit.Text = body.Text
			}
			if body.Enabled != nil {
				sit.Enabled = *body.Enabled
			}
			if err := s.Coordinator.UpdateSituation(r.Context(), sit, true); err != nil {
				s.writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, sit)
			return
		}
	}
	s.writeError(w, engine.ErrInvalidCommand)
}

// --- websocket ---

// handleWS upgrades the connection and registers a client for the room. It
// authenticates via the session cookie (player), the admin cookie (admin), or
// the ?screen=1 query (read-only screen). On connect it sends a SNAPSHOT.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	// Determine viewer type.
	isAdmin := s.isAdmin(r)
	isScreen := r.URL.Query().Get("screen") == "1"
	var playerID string
	if !isAdmin && !isScreen {
		token, err := s.sessionCookie(r)
		if err != nil {
			s.writeError(w, engine.ErrInvalidSession)
			return
		}
		sess, err := s.Sessions.Authenticate(r.Context(), token)
		if err != nil {
			s.writeError(w, engine.ErrInvalidSession)
			return
		}
		playerID = sess.PlayerID
	}

	room, err := s.Coordinator.GetRoomByCode(r.Context(), code)
	if err != nil {
		s.writeError(w, err)
		return
	}
	agg, err := s.Coordinator.LoadAggregate(r.Context(), room.ID)
	if err != nil {
		s.writeError(w, err)
		return
	}

	// A player with a valid session in another room must not join this room's
	// WebSocket stream.
	if playerID != "" {
		if p := agg.PlayerByID(playerID); p == nil || p.RoomID != room.ID {
			s.writeError(w, engine.ErrPlayerNotFound)
			return
		}
	}

	var snap *projection.GameSnapshot
	switch {
	case isAdmin:
		snap = projection.HostSnapshot(agg, "")
	case isScreen:
		snap = projection.ScreenSnapshot(agg)
	default:
		snap = projection.PlayerSnapshot(agg, playerID, false)
	}

	conn, err := coderwebsocket.Accept(w, r, &coderwebsocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}

	client := websocket.NewClient(conn, room.ID, s.Hub)
	if playerID != "" {
		client.OnDisconnect = func() {
			_ = s.Coordinator.MarkDisconnected(context.Background(), playerID)
		}
	}
	s.Hub.Register(client)

	initMsg, _ := json.Marshal(map[string]any{"type": "SNAPSHOT", "snapshot": snap})
	client.Send(initMsg)

	client.Run()
}

// roomSettingsFromDTO converts the settings DTO into a room.RoomSettings,
// preserving defaults for fields not sent by the client.
func roomSettingsFromDTO(d struct {
	MinPlayers                   int  `json:"minPlayers"`
	MaxPlayers                   int  `json:"maxPlayers"`
	HandSize                     int  `json:"handSize"`
	PreparationTimeoutSeconds    int  `json:"preparationTimeoutSeconds"`
	RoundSelectionTimeoutSeconds int  `json:"roundSelectionTimeoutSeconds"`
	VotingTimeoutSeconds         int  `json:"votingTimeoutSeconds"`
	InfiniteGame                 bool `json:"infiniteGame"`
}) room.RoomSettings {
	rs := room.DefaultRoomSettings()
	if d.MinPlayers != 0 {
		rs.MinPlayers = d.MinPlayers
	}
	if d.MaxPlayers != 0 {
		rs.MaxPlayers = d.MaxPlayers
	}
	if d.HandSize != 0 {
		rs.HandSize = d.HandSize
	}
	if d.PreparationTimeoutSeconds != 0 {
		rs.PreparationTimeoutSeconds = d.PreparationTimeoutSeconds
	}
	if d.RoundSelectionTimeoutSeconds != 0 {
		rs.RoundSelectionTimeoutSeconds = d.RoundSelectionTimeoutSeconds
	}
	if d.VotingTimeoutSeconds != 0 {
		rs.VotingTimeoutSeconds = d.VotingTimeoutSeconds
	}
	rs.InfiniteGame = d.InfiniteGame
	return rs
}
