// Package projection builds server-side, viewer-specific snapshots from an
// engine aggregate. Different clients receive different projections to enforce
// the game's information-hiding security invariants.
package projection

import (
	"sort"
	"time"

	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/engine"
)

// GameSnapshot is the JSON-friendly snapshot sent to a client.
type GameSnapshot struct {
	Revision   int             `json:"revision"`
	ServerTime string          `json:"serverTime"`
	Room       RoomDTO         `json:"room"`
	Game       *GameDTO        `json:"game,omitempty"`
	Players    []RoomPlayerDTO `json:"players"`
	Settings   GameSettingsDTO `json:"settings"`
	Phase      string          `json:"phase"`
	Actor      ActorDTO        `json:"actor"`
	PhaseData  map[string]any  `json:"phaseData"`
}

// RoomPlayerDTO is the public room-player representation. It is always present
// in a snapshot (even in LOBBY, before a game exists) so clients can show who
// has joined and the room settings.
type RoomPlayerDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Connected bool   `json:"connected"`
	IsHost    bool   `json:"isHost"`
}

// RoomDTO is the public room representation.
type RoomDTO struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	State    string `json:"state"`
	Revision int    `json:"revision"`
}

// GameDTO is the public game representation.
type GameDTO struct {
	ID                 string             `json:"id"`
	State              string             `json:"state"`
	CurrentCycleNumber int                `json:"currentCycleNumber"`
	Settings           GameSettingsDTO    `json:"settings"`
	Players            []GamePlayerDTO    `json:"players"`
	Leaderboard        []LeaderboardEntry `json:"leaderboard"`
}

// GameSettingsDTO is the JSON-friendly room/game settings.
type GameSettingsDTO struct {
	MinPlayers                   int            `json:"minPlayers"`
	MaxPlayers                   int            `json:"maxPlayers"`
	HandSize                     int            `json:"handSize"`
	PreparationTimeoutSeconds    int            `json:"preparationTimeoutSeconds"`
	RoundSelectionTimeoutSeconds int            `json:"roundSelectionTimeoutSeconds"`
	VotingTimeoutSeconds         int            `json:"votingTimeoutSeconds"`
	InfiniteGame                 bool           `json:"infiniteGame"`
	SituationSeparator           string         `json:"situationSeparator"`
	ScoreConfig                  ScoreConfigDTO `json:"scoreConfig"`
}

// ScoreConfigDTO mirrors scoring.ScoreConfig for JSON output.
type ScoreConfigDTO struct {
	AllGuessedActivePlayer  int `json:"allGuessedActivePlayer"`
	AllGuessedGuesser       int `json:"allGuessedGuesser"`
	NoneGuessedActivePlayer int `json:"noneGuessedActivePlayer"`
	NoneGuessedOtherPlayer  int `json:"noneGuessedOtherPlayer"`
	PartialActiveBase       int `json:"partialActiveBase"`
	PartialActivePerGuesser int `json:"partialActivePerGuesser"`
	PartialGuesser          int `json:"partialGuesser"`
	VoteForSubmittedMeme    int `json:"voteForSubmittedMeme"`
}

// GamePlayerDTO is the public game-player representation.
type GamePlayerDTO struct {
	ID                  string `json:"id"`
	PlayerID            string `json:"playerId"`
	DisplayName         string `json:"displayName"`
	TurnOrder           int    `json:"turnOrder"`
	Score               int    `json:"score"`
	ParticipationStatus string `json:"participationStatus"`
	Connected           bool   `json:"connected"`
}

// LeaderboardEntry is a single row of the score leaderboard.
type LeaderboardEntry struct {
	GamePlayerID string `json:"gamePlayerId"`
	DisplayName  string `json:"displayName"`
	Score        int    `json:"score"`
}

// ActorDTO describes the acting viewer.
type ActorDTO struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsHost   bool   `json:"isHost"`
	IsAdmin  bool   `json:"isAdmin"`
}

// VoteOptionDTO is a vote option without owner/original information (used
// during ROUND_VOTING to preserve anonymity).
type VoteOptionDTO struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	MemeID string `json:"memeId"`
}

// RevealVoteOptionDTO is a fully revealed vote option (ROUND_RESULTS).
type RevealVoteOptionDTO struct {
	ID                string `json:"id"`
	Number            int    `json:"number"`
	MemeID            string `json:"memeId"`
	OwnerGamePlayerID string `json:"ownerGamePlayerId"`
	IsOriginal        bool   `json:"isOriginal"`
	Votes             int    `json:"votes"`
}

// SubmissionDTO is a revealed round submission.
type SubmissionDTO struct {
	GamePlayerID string `json:"gamePlayerId"`
	DisplayName  string `json:"displayName"`
	MemeID       string `json:"memeId"`
}

// ScoreDeltaDTO is a revealed per-player score change.
type ScoreDeltaDTO struct {
	GamePlayerID string `json:"gamePlayerId"`
	DisplayName  string `json:"displayName"`
	Delta        int    `json:"delta"`
	NewScore     int    `json:"newScore"`
}

// viewerKind identifies the projection audience.
type viewerKind int

const (
	viewerPlayer viewerKind = iota
	viewerScreen
	viewerHost
)

// viewer describes who is receiving the snapshot.
type viewer struct {
	kind     viewerKind
	playerID string
	isAdmin  bool
}

// PlayerSnapshot builds the snapshot for a specific player, including their
// private data (hand, prepared turn, vote options with their forbidden option,
// and their own submission). It never reveals other players' private data.
func PlayerSnapshot(agg *engine.Aggregate, playerID string, isAdmin bool) *GameSnapshot {
	return buildSnapshot(agg, viewer{kind: viewerPlayer, playerID: playerID, isAdmin: isAdmin})
}

// ScreenSnapshot builds the public screen snapshot. It contains no session
// data and no private hands.
func ScreenSnapshot(agg *engine.Aggregate) *GameSnapshot {
	return buildSnapshot(agg, viewer{kind: viewerScreen})
}

// HostSnapshot builds the admin snapshot. It includes room settings, the
// player list with connection status, and admin controls availability, but
// must not reveal hidden answers before ROUND_RESULTS.
func HostSnapshot(agg *engine.Aggregate, adminPlayerID string) *GameSnapshot {
	return buildSnapshot(agg, viewer{kind: viewerHost, playerID: adminPlayerID, isAdmin: true})
}

func buildSnapshot(agg *engine.Aggregate, v viewer) *GameSnapshot {
	snap := &GameSnapshot{
		Revision:   agg.Room.Revision,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		Room:       roomDTO(agg.Room),
		Players:    roomPlayersDTO(agg),
		Settings:   settingsDTO(agg.Settings),
		Phase:      string(agg.Phase),
		Actor:      actorDTO(agg, v),
		PhaseData:  map[string]any{},
	}
	if agg.Game != nil {
		snap.Game = gameDTO(agg)
	}
	snap.PhaseData = phaseData(agg, v)
	return snap
}

func roomDTO(r *room.Room) RoomDTO {
	return RoomDTO{ID: r.ID, Code: r.Code, State: string(r.State), Revision: r.Revision}
}

// roomPlayersDTO returns the room players sorted by join time. It is populated
// regardless of whether a game has started so the LOBBY view can show who has
// joined. Players who have left or been kicked (LeftAt set) are excluded so
// they disappear from every client's player list.
func roomPlayersDTO(agg *engine.Aggregate) []RoomPlayerDTO {
	sorted := append([]*player.Player(nil), agg.Players...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].JoinedAt.Before(sorted[j].JoinedAt) })
	out := make([]RoomPlayerDTO, 0, len(sorted))
	for _, p := range sorted {
		if p.LeftAt != nil {
			continue
		}
		out = append(out, RoomPlayerDTO{
			ID:        p.ID,
			Name:      p.Name,
			Role:      string(p.Role),
			Connected: p.Connected,
			IsHost:    p.Role == player.RoleHost,
		})
	}
	return out
}

func actorDTO(agg *engine.Aggregate, v viewer) ActorDTO {
	a := ActorDTO{IsAdmin: v.isAdmin}
	if v.playerID != "" {
		if p := agg.PlayerByID(v.playerID); p != nil {
			a.PlayerID = p.ID
			a.Name = p.Name
			a.Role = string(p.Role)
			a.IsHost = p.Role == player.RoleHost
		}
	}
	return a
}

func gameDTO(agg *engine.Aggregate) *GameDTO {
	g := &GameDTO{
		ID:          agg.Game.ID,
		State:       string(agg.Game.State),
		Settings:    settingsDTO(agg.Settings),
		Players:     []GamePlayerDTO{},
		Leaderboard: []LeaderboardEntry{},
	}
	if agg.Cycle != nil {
		g.CurrentCycleNumber = agg.Cycle.Number
	}
	for _, gp := range agg.GamePlayers {
		connected := false
		if p := agg.PlayerByID(gp.PlayerID); p != nil {
			connected = p.Connected
		}
		g.Players = append(g.Players, GamePlayerDTO{
			ID:                  gp.ID,
			PlayerID:            gp.PlayerID,
			DisplayName:         gp.DisplayName,
			TurnOrder:           gp.TurnOrder,
			Score:               gp.Score,
			ParticipationStatus: string(gp.ParticipationStatus),
			Connected:           connected,
		})
	}
	g.Leaderboard = leaderboard(agg)
	return g
}

func settingsDTO(s room.RoomSettings) GameSettingsDTO {
	return GameSettingsDTO{
		MinPlayers:                   s.MinPlayers,
		MaxPlayers:                   s.MaxPlayers,
		HandSize:                     s.HandSize,
		PreparationTimeoutSeconds:    s.PreparationTimeoutSeconds,
		RoundSelectionTimeoutSeconds: s.RoundSelectionTimeoutSeconds,
		VotingTimeoutSeconds:         s.VotingTimeoutSeconds,
		InfiniteGame:                 s.InfiniteGame,
		SituationSeparator:           s.SituationSeparator,
		ScoreConfig: ScoreConfigDTO{
			AllGuessedActivePlayer:  s.ScoreConfig.AllGuessedActivePlayer,
			AllGuessedGuesser:       s.ScoreConfig.AllGuessedGuesser,
			NoneGuessedActivePlayer: s.ScoreConfig.NoneGuessedActivePlayer,
			NoneGuessedOtherPlayer:  s.ScoreConfig.NoneGuessedOtherPlayer,
			PartialActiveBase:       s.ScoreConfig.PartialActiveBase,
			PartialActivePerGuesser: s.ScoreConfig.PartialActivePerGuesser,
			PartialGuesser:          s.ScoreConfig.PartialGuesser,
			VoteForSubmittedMeme:    s.ScoreConfig.VoteForSubmittedMeme,
		},
	}
}

func leaderboard(agg *engine.Aggregate) []LeaderboardEntry {
	sorted := append([]*game.GamePlayer(nil), agg.GamePlayers...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	out := make([]LeaderboardEntry, 0, len(sorted))
	for _, gp := range sorted {
		out = append(out, LeaderboardEntry{GamePlayerID: gp.ID, DisplayName: gp.DisplayName, Score: gp.Score})
	}
	return out
}

// phaseData fills the per-phase, per-viewer data. It is the security-critical
// function: hidden information is only revealed to the appropriate viewer and
// only in the appropriate phase.
func phaseData(agg *engine.Aggregate, v viewer) map[string]any {
	data := map[string]any{}
	switch agg.Phase {
	case game.PhasePreparation:
		data["preparedCount"] = countPrepared(agg)
		data["totalPlayers"] = len(agg.ActiveGamePlayers())
		if v.kind == viewerPlayer {
			if gp := agg.GamePlayerByPlayerID(v.playerID); gp != nil {
				if hand := agg.HandFor(gp.ID, engine.HandKindPreparation); hand != nil {
					data["hand"] = hand.MemeIDs
				}
				if pt := agg.PreparedTurnFor(gp.ID); pt != nil {
					data["preparedTurn"] = map[string]any{
						"situationText": pt.SituationText,
						"memeId":        pt.OriginalMemeID,
					}
				}
			}
		}

	case game.PhaseRoundSelection:
		if r := agg.CurrentRound(); r != nil {
			data["situationText"] = r.SituationText
			data["activeGamePlayerId"] = r.ActiveGamePlayerID
		}
		if v.kind == viewerPlayer {
			if gp := agg.GamePlayerByPlayerID(v.playerID); gp != nil {
				if hand := agg.HandFor(gp.ID, engine.HandKindRound); hand != nil {
					data["hand"] = hand.MemeIDs
				}
				if r := agg.CurrentRound(); r != nil && hasSubmission(agg, r.ID, gp.ID) {
					data["submitted"] = true
				}
			}
		}

	case game.PhaseRoundVoting:
		if r := agg.CurrentRound(); r != nil {
			data["situationText"] = r.SituationText
			data["voteOptions"] = voteOptionsDTO(agg, r)
			if v.kind == viewerPlayer {
				if gp := agg.GamePlayerByPlayerID(v.playerID); gp != nil {
					for _, o := range agg.VoteOptions {
						if o.RoundID == r.ID && o.OwnerGamePlayerID == gp.ID {
							data["forbiddenOptionId"] = o.ID
							break
						}
					}
				}
			}
		}

	case game.PhaseRoundResults:
		data["reveal"] = revealDTO(agg)

	case game.PhaseCycleResults:
		if agg.Cycle != nil {
			data["cycleNumber"] = agg.Cycle.Number
		}
		data["leaderboard"] = leaderboard(agg)

	case game.PhaseGameResults:
		data["leaderboard"] = leaderboard(agg)
	}
	return data
}

func countPrepared(agg *engine.Aggregate) int {
	n := 0
	for _, gp := range agg.ActiveGamePlayers() {
		if agg.PreparedTurnFor(gp.ID) != nil {
			n++
		}
	}
	return n
}

func hasSubmission(agg *engine.Aggregate, roundID, gamePlayerID string) bool {
	for _, s := range agg.Submissions {
		if s.RoundID == roundID && s.GamePlayerID == gamePlayerID {
			return true
		}
	}
	return false
}

// voteOptionsDTO returns vote options without owner/original/votes. This is
// used for every viewer during ROUND_VOTING to preserve anonymity.
func voteOptionsDTO(agg *engine.Aggregate, r *round.Round) []VoteOptionDTO {
	var out []VoteOptionDTO
	for _, o := range agg.VoteOptions {
		if o.RoundID != r.ID {
			continue
		}
		out = append(out, VoteOptionDTO{ID: o.ID, Number: o.Number, MemeID: o.MemeID})
	}
	return out
}

// revealDTO returns the full round reveal for ROUND_RESULTS.
func revealDTO(agg *engine.Aggregate) map[string]any {
	r := agg.CurrentRound()
	if r == nil {
		return map[string]any{}
	}
	votesByOption := map[string]int{}
	for _, v := range agg.Votes {
		if v.RoundID == r.ID {
			votesByOption[v.VoteOptionID]++
		}
	}
	options := []RevealVoteOptionDTO{}
	for _, o := range agg.VoteOptions {
		if o.RoundID != r.ID {
			continue
		}
		options = append(options, RevealVoteOptionDTO{
			ID:                o.ID,
			Number:            o.Number,
			MemeID:            o.MemeID,
			OwnerGamePlayerID: o.OwnerGamePlayerID,
			IsOriginal:        o.IsOriginal,
			Votes:             votesByOption[o.ID],
		})
	}
	submissions := []SubmissionDTO{}
	for _, s := range agg.Submissions {
		if s.RoundID != r.ID {
			continue
		}
		name := ""
		if gp := agg.GamePlayerByID(s.GamePlayerID); gp != nil {
			name = gp.DisplayName
		}
		submissions = append(submissions, SubmissionDTO{GamePlayerID: s.GamePlayerID, DisplayName: name, MemeID: s.MemeID})
	}
	deltas := []ScoreDeltaDTO{}
	for _, rs := range agg.RoundScores {
		if rs.RoundID != r.ID {
			continue
		}
		name := ""
		if gp := agg.GamePlayerByID(rs.GamePlayerID); gp != nil {
			name = gp.DisplayName
		}
		deltas = append(deltas, ScoreDeltaDTO{GamePlayerID: rs.GamePlayerID, DisplayName: name, Delta: rs.Delta, NewScore: rs.NewScore})
	}
	return map[string]any{
		"situationText":      r.SituationText,
		"originalMemeId":     r.OriginalMemeID,
		"activeGamePlayerId": r.ActiveGamePlayerID,
		"voteOptions":        options,
		"submissions":        submissions,
		"scoreDeltas":        deltas,
		"leaderboard":        leaderboard(agg),
	}
}
