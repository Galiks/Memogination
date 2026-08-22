package engine

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/memomarium/memomarium/internal/domain/scoring"
)

// --- Authorization helpers ---

// isHostOrAdmin reports whether the acting player is the room host or a Local
// Admin.
func (e *Engine) isHostOrAdmin(agg *Aggregate, cmd Command) bool {
	if cmd.IsAdmin {
		return true
	}
	p := agg.PlayerByID(cmd.PlayerID)
	return p != nil && p.Role == player.RoleHost
}

// --- Phase helpers ---

// setPhaseDeadline sets the aggregate phase deadline based on the phase and
// the game settings snapshot (or room settings before a game exists).
func (e *Engine) setPhaseDeadline(agg *Aggregate, phase game.GamePhase, now time.Time) {
	var timeout int
	switch phase {
	case game.PhasePreparation:
		timeout = agg.Settings.PreparationTimeoutSeconds
	case game.PhaseRoundSelection:
		timeout = agg.Settings.RoundSelectionTimeoutSeconds
	case game.PhaseRoundVoting:
		timeout = agg.Settings.VotingTimeoutSeconds
	}
	if timeout > 0 {
		d := now.Add(time.Duration(timeout) * time.Second)
		agg.PhaseDeadlineAt = &d
	} else {
		agg.PhaseDeadlineAt = nil
	}
}

// setPhase sets the aggregate phase and its deadline.
func (e *Engine) setPhase(agg *Aggregate, phase game.GamePhase, now time.Time) {
	agg.Phase = phase
	e.setPhaseDeadline(agg, phase, now)
}

// --- Round helpers ---

// nextEligibleActivePlayer returns the ACTIVE game player with the lowest
// turn order who has not yet been the active player this cycle.
func (e *Engine) nextEligibleActivePlayer(agg *Aggregate) *game.GamePlayer {
	activeThisCycle := map[string]bool{}
	if agg.Cycle != nil {
		for _, r := range agg.Rounds {
			if r.CycleID == agg.Cycle.ID {
				activeThisCycle[r.ActiveGamePlayerID] = true
			}
		}
	}
	var best *game.GamePlayer
	for _, gp := range agg.ActiveGamePlayers() {
		if activeThisCycle[gp.ID] {
			continue
		}
		if best == nil || gp.TurnOrder < best.TurnOrder {
			best = gp
		}
	}
	return best
}

// startRound creates a new round for the given active game player, deals round
// hands, and moves the aggregate to ROUND_SELECTION.
func (e *Engine) startRound(agg *Aggregate, activeGP *game.GamePlayer, now time.Time) (*round.Round, error) {
	// SKIPPED is per-round only: a player who timed out in a previous round
	// (or cycle) must be able to participate again.
	resetSkipped(agg)
	pt := agg.PreparedTurnFor(activeGP.ID)
	if pt == nil {
		return nil, ErrInternal
	}
	r := &round.Round{
		ID:                 uuid.NewString(),
		GameID:             agg.Game.ID,
		CycleID:            agg.Cycle.ID,
		ActiveGamePlayerID: activeGP.ID,
		Phase:              string(game.PhaseRoundSelection),
		SituationText:      pt.SituationText,
		OriginalMemeID:     pt.OriginalMemeID,
		Status:             "ACTIVE",
		StartedAt:          now,
	}
	if agg.Settings.RoundSelectionTimeoutSeconds > 0 {
		d := now.Add(time.Duration(agg.Settings.RoundSelectionTimeoutSeconds) * time.Second)
		r.DeadlineAt = &d
	}
	agg.Rounds = append(agg.Rounds, r)
	agg.Game.CurrentRoundID = r.ID
	if err := e.dealer.DealRoundHands(agg, r); err != nil {
		return nil, err
	}
	e.setPhase(agg, game.PhaseRoundSelection, now)
	return r, nil
}

// advanceToRoundSelection starts the first round of the cycle.
func (e *Engine) advanceToRoundSelection(agg *Aggregate, now time.Time) ([]Event, error) {
	activeGP := e.nextEligibleActivePlayer(agg)
	if activeGP == nil {
		return nil, ErrInternal
	}
	r, err := e.startRound(agg, activeGP, now)
	if err != nil {
		return nil, err
	}
	return []Event{
		{Type: EventRoundStarted, Revision: agg.Room.Revision, Data: map[string]any{"roundId": r.ID, "activeGamePlayerId": activeGP.ID}},
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}, nil
}

// advanceToRoundVoting builds vote options and moves to ROUND_VOTING.
func (e *Engine) advanceToRoundVoting(agg *Aggregate, r *round.Round, now time.Time) []Event {
	options := make([]*round.VoteOption, 0, len(agg.Submissions)+1)
	options = append(options, &round.VoteOption{
		ID:                uuid.NewString(),
		RoundID:           r.ID,
		MemeID:            r.OriginalMemeID,
		OwnerGamePlayerID: r.ActiveGamePlayerID,
		IsOriginal:        true,
	})
	for _, s := range agg.Submissions {
		if s.RoundID != r.ID {
			continue
		}
		options = append(options, &round.VoteOption{
			ID:                uuid.NewString(),
			RoundID:           r.ID,
			MemeID:            s.MemeID,
			OwnerGamePlayerID: s.GamePlayerID,
			IsOriginal:        false,
		})
	}
	Shuffle(e.dealer.rng, options)
	for i, o := range options {
		o.Number = i + 1
	}
	agg.VoteOptions = append(agg.VoteOptions, options...)
	e.setPhase(agg, game.PhaseRoundVoting, now)
	return []Event{
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}
}

// advanceToRoundResults computes scores and moves to ROUND_RESULTS.
func (e *Engine) advanceToRoundResults(agg *Aggregate, r *round.Round, now time.Time) []Event {
	votes := make([]*round.Vote, 0)
	for _, v := range agg.Votes {
		if v.RoundID == r.ID {
			votes = append(votes, v)
		}
	}

	optionByID := map[string]*round.VoteOption{}
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID {
			optionByID[o.ID] = o
		}
	}

	votesForOriginal := 0
	guesserIDs := make([]string, 0, len(votes))
	voteForSubmitted := map[string]bool{}
	for _, v := range votes {
		guesserIDs = append(guesserIDs, v.GamePlayerID)
		opt := optionByID[v.VoteOptionID]
		if opt == nil {
			continue
		}
		if opt.IsOriginal {
			votesForOriginal++
		}
		if opt.OwnerGamePlayerID == v.GamePlayerID {
			voteForSubmitted[v.GamePlayerID] = true
		}
	}

	facts := scoring.RoundFacts{
		ActivePlayerID:       r.ActiveGamePlayerID,
		OriginalOwnerID:      r.ActiveGamePlayerID,
		TotalVotes:           len(votes),
		VotesForOriginal:     votesForOriginal,
		GuesserIDs:           guesserIDs,
		VoteForSubmittedMeme: voteForSubmitted,
	}

	cfg := agg.Settings.ScoreConfig
	if agg.Game != nil {
		cfg = agg.Game.SettingsSnapshot.ScoreConfig
	}
	deltas := (scoring.ScoreCalculator{}).CalculateRound(facts, cfg)
	for _, d := range deltas {
		gp := agg.GamePlayerByID(d.GamePlayerID)
		if gp == nil {
			continue
		}
		prev := gp.Score
		gp.Score += d.Delta
		agg.RoundScores = append(agg.RoundScores, &round.RoundScore{
			RoundID:       r.ID,
			GamePlayerID:  gp.ID,
			PreviousScore: prev,
			Delta:         d.Delta,
			NewScore:      gp.Score,
		})
	}

	r.Status = "FINISHED"
	r.FinishedAt = &now
	e.setPhase(agg, game.PhaseRoundResults, now)
	return []Event{
		{Type: EventRoundFinished, Revision: agg.Room.Revision, Data: map[string]any{"roundId": r.ID}},
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}
}

// finishCycle ends the current cycle, either moving to CYCLE_RESULTS (infinite
// mode) or finishing the game.
func (e *Engine) finishCycle(agg *Aggregate, now time.Time) []Event {
	nowT := now
	agg.Cycle.FinishedAt = &nowT
	if agg.Settings.InfiniteGame {
		e.setPhase(agg, game.PhaseCycleResults, now)
		return []Event{
			{Type: EventCycleFinished, Revision: agg.Room.Revision, Data: map[string]any{"cycleId": agg.Cycle.ID}},
			{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
		}
	}
	agg.Game.State = game.StateFinished
	agg.Game.FinishedAt = &nowT
	agg.Room.State = room.StateClosed
	agg.Room.ClosedAt = &nowT
	e.setPhase(agg, game.PhaseGameResults, now)
	return []Event{
		{Type: EventGameFinished, Revision: agg.Room.Revision, Data: map[string]any{"gameId": agg.Game.ID}},
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}
}

// --- Submission / vote helpers ---

// resetSkipped returns any game player whose participation was SKIPPED (e.g. a
// voting timeout) back to ACTIVE. SKIPPED is per-round/per-cycle only.
func resetSkipped(agg *Aggregate) {
	for _, gp := range agg.GamePlayers {
		if gp.ParticipationStatus == game.ParticipationSkipped {
			gp.ParticipationStatus = game.ParticipationActive
		}
	}
}

func hasSubmission(agg *Aggregate, roundID, gamePlayerID string) bool {
	for _, s := range agg.Submissions {
		if s.RoundID == roundID && s.GamePlayerID == gamePlayerID {
			return true
		}
	}
	return false
}

func hasVoted(agg *Aggregate, roundID, gamePlayerID string) bool {
	for _, v := range agg.Votes {
		if v.RoundID == roundID && v.GamePlayerID == gamePlayerID {
			return true
		}
	}
	return false
}

func allActivePrepared(agg *Aggregate) bool {
	for _, gp := range agg.ActiveGamePlayers() {
		if agg.PreparedTurnFor(gp.ID) == nil {
			return false
		}
	}
	return true
}

func allNonActiveSubmitted(agg *Aggregate, r *round.Round) bool {
	for _, gp := range agg.ActiveGamePlayers() {
		if gp.ID == r.ActiveGamePlayerID {
			continue
		}
		if !hasSubmission(agg, r.ID, gp.ID) {
			return false
		}
	}
	return true
}

func allNonActiveVoted(agg *Aggregate, r *round.Round) bool {
	for _, gp := range agg.ActiveGamePlayers() {
		if gp.ID == r.ActiveGamePlayerID {
			continue
		}
		if !hasVoted(agg, r.ID, gp.ID) {
			return false
		}
	}
	return true
}

func removePreparedTurn(agg *Aggregate, gamePlayerID string) {
	kept := agg.PreparedTurns[:0]
	for _, pt := range agg.PreparedTurns {
		if pt.GamePlayerID != gamePlayerID {
			kept = append(kept, pt)
		}
	}
	agg.PreparedTurns = kept
}

func removeSubmission(agg *Aggregate, roundID, gamePlayerID string) {
	kept := agg.Submissions[:0]
	for _, s := range agg.Submissions {
		if s.RoundID == roundID && s.GamePlayerID == gamePlayerID {
			continue
		}
		kept = append(kept, s)
	}
	agg.Submissions = kept
}

// --- START_GAME ---

func (e *Engine) startGame(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Room == nil {
		return nil, ErrRoomNotFound
	}
	if !e.isHostOrAdmin(agg, cmd) {
		return nil, ErrNotAllowed
	}
	if agg.Room.State != room.StateLobby {
		return nil, ErrGameAlreadyStarted
	}

	activePlayers := agg.ActivePlayers()
	if len(activePlayers) < agg.Settings.MinPlayers {
		return nil, wrapf(ErrInvalidCommand, "need at least %d players to start, have %d", agg.Settings.MinPlayers, len(activePlayers))
	}

	n := len(activePlayers)
	h := agg.Settings.HandSize
	requiredMemes := n * h
	if roundReq := n + (n-1)*h; roundReq > requiredMemes {
		requiredMemes = roundReq
	}
	enabledMemes := len(agg.EnabledMemes())
	if enabledMemes < requiredMemes {
		return nil, wrapf(ErrNotEnoughMemes,
			"Для %d игроков с рукой %d требуется минимум %d активных мемов. Доступно: %d.",
			n, h, requiredMemes, enabledMemes)
	}
	if len(agg.EnabledSituations()) < 1 {
		return nil, ErrNotEnoughSituations
	}

	now := cmd.Now
	gameID := uuid.NewString()
	agg.Game = &game.Game{
		ID:               gameID,
		RoomID:           agg.Room.ID,
		State:            game.StateActive,
		Revision:         0,
		SettingsSnapshot: game.NewGameSettingsSnapshot(agg.Settings),
		StartedAt:        now,
	}

	// Game players ordered by joinedAt ascending.
	sorted := append([]*player.Player(nil), activePlayers...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].JoinedAt.Before(sorted[j-1].JoinedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	for i, p := range sorted {
		agg.GamePlayers = append(agg.GamePlayers, &game.GamePlayer{
			ID:                  uuid.NewString(),
			GameID:              gameID,
			PlayerID:            p.ID,
			DisplayName:         p.Name,
			TurnOrder:           i,
			Score:               0,
			ParticipationStatus: game.ParticipationActive,
		})
	}

	cycleID := uuid.NewString()
	agg.Cycle = &game.GameCycle{
		ID:        cycleID,
		GameID:    gameID,
		Number:    1,
		StartedAt: now,
	}
	agg.Game.CurrentCycleID = cycleID

	agg.Room.State = room.StateInGame

	if err := e.dealer.DealPreparationHands(agg); err != nil {
		return nil, err
	}
	e.setPhase(agg, game.PhasePreparation, now)

	return []Event{
		{Type: EventGameStarted, Revision: agg.Room.Revision, Data: map[string]any{"gameId": gameID, "cycleId": cycleID}},
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}, nil
}

// --- SUBMIT_PREPARATION ---

func (e *Engine) submitPreparation(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Phase != game.PhasePreparation {
		return nil, ErrInvalidPhase
	}
	gp := agg.GamePlayerByPlayerID(cmd.PlayerID)
	if gp == nil || gp.ParticipationStatus != game.ParticipationActive {
		return nil, ErrNotAllowed
	}

	situationText, _ := cmd.Payload["situationText"].(string)
	memeID, _ := cmd.Payload["memeId"].(string)
	trimmed := strings.TrimSpace(situationText)
	if n := len([]rune(trimmed)); n < 1 || n > 500 {
		return nil, wrapf(ErrInvalidCommand, "situationText must be 1..500 characters, got %d", n)
	}

	hand := agg.HandFor(gp.ID, HandKindPreparation)
	if hand == nil || !contains(hand.MemeIDs, memeID) {
		return nil, ErrInvalidMeme
	}
	if agg.PreparedTurnFor(gp.ID) != nil {
		return nil, ErrCommandAlreadyProcessed
	}

	agg.PreparedTurns = append(agg.PreparedTurns, &round.PreparedTurn{
		ID:             uuid.NewString(),
		CycleID:        agg.Cycle.ID,
		GamePlayerID:   gp.ID,
		SituationText:  trimmed,
		OriginalMemeID: memeID,
		Status:         "READY",
		CreatedAt:      cmd.Now,
	})

	events := []Event{
		{Type: EventPreparationSubmit, Revision: agg.Room.Revision, Data: map[string]any{"gamePlayerId": gp.ID}},
	}
	if allActivePrepared(agg) {
		evs, err := e.advanceToRoundSelection(agg, cmd.Now)
		if err != nil {
			return nil, err
		}
		events = append(events, evs...)
	}
	return events, nil
}

// --- SUBMIT_ROUND_MEME ---

func (e *Engine) submitRoundMeme(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Phase != game.PhaseRoundSelection {
		return nil, ErrInvalidPhase
	}
	gp := agg.GamePlayerByPlayerID(cmd.PlayerID)
	if gp == nil || gp.ParticipationStatus != game.ParticipationActive {
		return nil, ErrNotAllowed
	}
	r := agg.CurrentRound()
	if r == nil {
		return nil, ErrInvalidPhase
	}
	if r.ActiveGamePlayerID == gp.ID {
		return nil, ErrNotAllowed
	}

	memeID, _ := cmd.Payload["memeId"].(string)
	hand := agg.HandFor(gp.ID, HandKindRound)
	if hand == nil || !contains(hand.MemeIDs, memeID) {
		return nil, ErrInvalidMeme
	}
	if hasSubmission(agg, r.ID, gp.ID) {
		return nil, ErrCommandAlreadyProcessed
	}

	agg.Submissions = append(agg.Submissions, &round.RoundSubmission{
		ID:           uuid.NewString(),
		RoundID:      r.ID,
		GamePlayerID: gp.ID,
		MemeID:       memeID,
		CreatedAt:    cmd.Now,
	})

	events := []Event{
		{Type: EventRoundSubmitted, Revision: agg.Room.Revision, Data: map[string]any{"roundId": r.ID, "gamePlayerId": gp.ID, "memeId": memeID}},
	}
	if allNonActiveSubmitted(agg, r) {
		events = append(events, e.advanceToRoundVoting(agg, r, cmd.Now)...)
	}
	return events, nil
}

// --- SUBMIT_VOTE ---

func (e *Engine) submitVote(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Phase != game.PhaseRoundVoting {
		return nil, ErrInvalidPhase
	}
	gp := agg.GamePlayerByPlayerID(cmd.PlayerID)
	if gp == nil || gp.ParticipationStatus != game.ParticipationActive {
		return nil, ErrNotAllowed
	}
	r := agg.CurrentRound()
	if r == nil {
		return nil, ErrInvalidPhase
	}
	if r.ActiveGamePlayerID == gp.ID {
		return nil, ErrNotAllowed
	}

	voteOptionID, _ := cmd.Payload["voteOptionId"].(string)
	var opt *round.VoteOption
	for _, o := range agg.VoteOptions {
		if o.ID == voteOptionID {
			opt = o
			break
		}
	}
	if opt == nil || opt.RoundID != r.ID {
		return nil, ErrInvalidVote
	}
	if opt.OwnerGamePlayerID == gp.ID {
		return nil, ErrOwnMemeVoteForbidden
	}
	if hasVoted(agg, r.ID, gp.ID) {
		return nil, ErrCommandAlreadyProcessed
	}

	agg.Votes = append(agg.Votes, &round.Vote{
		ID:           uuid.NewString(),
		RoundID:      r.ID,
		GamePlayerID: gp.ID,
		VoteOptionID: voteOptionID,
		CreatedAt:    cmd.Now,
	})

	events := []Event{
		{Type: EventVoteSubmitted, Revision: agg.Room.Revision, Data: map[string]any{"roundId": r.ID, "gamePlayerId": gp.ID, "voteOptionId": voteOptionID}},
	}
	if allNonActiveVoted(agg, r) {
		events = append(events, e.advanceToRoundResults(agg, r, cmd.Now)...)
	}
	return events, nil
}

// --- NEXT_ROUND ---

func (e *Engine) nextRound(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Phase != game.PhaseRoundResults {
		return nil, ErrInvalidPhase
	}
	if !e.isHostOrAdmin(agg, cmd) {
		return nil, ErrNotAllowed
	}

	// SKIPPED is per-round only: reset before selecting the next active player
	// so a previously-skipped player can be active again.
	resetSkipped(agg)

	next := e.nextEligibleActivePlayer(agg)
	if next != nil {
		r, err := e.startRound(agg, next, cmd.Now)
		if err != nil {
			return nil, err
		}
		return []Event{
			{Type: EventRoundStarted, Revision: agg.Room.Revision, Data: map[string]any{"roundId": r.ID, "activeGamePlayerId": next.ID}},
			{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
		}, nil
	}

	return e.finishCycle(agg, cmd.Now), nil
}

// --- START_NEXT_CYCLE ---

func (e *Engine) startNextCycle(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if agg.Phase != game.PhaseCycleResults {
		return nil, ErrInvalidPhase
	}
	if !e.isHostOrAdmin(agg, cmd) {
		return nil, ErrNotAllowed
	}

	now := cmd.Now
	cycleID := uuid.NewString()
	agg.Cycle = &game.GameCycle{
		ID:        cycleID,
		GameID:    agg.Game.ID,
		Number:    agg.Cycle.Number + 1,
		StartedAt: now,
	}
	agg.Game.CurrentCycleID = cycleID

	// Reset per-cycle state; scores are preserved on GamePlayers.
	agg.PreparedTurns = nil
	agg.Rounds = nil
	agg.Hands = nil
	agg.Submissions = nil
	agg.VoteOptions = nil
	agg.Votes = nil
	agg.RoundScores = nil

	// SKIPPED is per-cycle too: players who timed out in the previous cycle
	// must be able to participate again.
	resetSkipped(agg)

	if err := e.dealer.DealPreparationHands(agg); err != nil {
		return nil, err
	}
	e.setPhase(agg, game.PhasePreparation, now)

	return []Event{
		{Type: EventCycleStarted, Revision: agg.Room.Revision, Data: map[string]any{"cycleId": cycleID, "number": agg.Cycle.Number}},
		{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
	}, nil
}

// --- FORCE_RESOLVE_PHASE / TIMEOUT_PHASE ---

func (e *Engine) forceResolvePhase(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if !e.isHostOrAdmin(agg, cmd) {
		return nil, ErrNotAllowed
	}
	events, err := e.resolvePhase(agg, cmd.Now)
	if err != nil {
		return nil, err
	}
	return append([]Event{{Type: EventForceResolved, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}}}, events...), nil
}

func (e *Engine) timeoutPhase(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	events, err := e.resolvePhase(agg, cmd.Now)
	if err != nil {
		return nil, err
	}
	return append([]Event{{Type: EventTimeoutApplied, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}}}, events...), nil
}

// resolvePhase applies the timeout rules for the current phase and returns the
// resulting transition events.
func (e *Engine) resolvePhase(agg *Aggregate, now time.Time) ([]Event, error) {
	switch agg.Phase {
	case game.PhasePreparation:
		for _, gp := range agg.ActiveGamePlayers() {
			if agg.PreparedTurnFor(gp.ID) != nil {
				continue
			}
			hand := agg.HandFor(gp.ID, HandKindPreparation)
			if hand == nil {
				continue
			}
			agg.PreparedTurns = append(agg.PreparedTurns, &round.PreparedTurn{
				ID:             uuid.NewString(),
				CycleID:        agg.Cycle.ID,
				GamePlayerID:   gp.ID,
				SituationText:  e.dealer.RandomSituation(agg),
				OriginalMemeID: e.dealer.RandomMemeFromHand(hand),
				Status:         "READY",
				CreatedAt:      now,
			})
		}
		return e.advanceToRoundSelection(agg, now)

	case game.PhaseRoundSelection:
		r := agg.CurrentRound()
		if r == nil {
			return nil, ErrInvalidPhase
		}
		for _, gp := range agg.ActiveGamePlayers() {
			if gp.ID == r.ActiveGamePlayerID {
				continue
			}
			if hasSubmission(agg, r.ID, gp.ID) {
				continue
			}
			hand := agg.HandFor(gp.ID, HandKindRound)
			if hand == nil {
				continue
			}
			agg.Submissions = append(agg.Submissions, &round.RoundSubmission{
				ID:           uuid.NewString(),
				RoundID:      r.ID,
				GamePlayerID: gp.ID,
				MemeID:       e.dealer.RandomMemeFromHand(hand),
				CreatedAt:    now,
			})
		}
		return e.advanceToRoundVoting(agg, r, now), nil

	case game.PhaseRoundVoting:
		r := agg.CurrentRound()
		if r == nil {
			return nil, ErrInvalidPhase
		}
		for _, gp := range agg.ActiveGamePlayers() {
			if gp.ID == r.ActiveGamePlayerID {
				continue
			}
			if hasVoted(agg, r.ID, gp.ID) {
				continue
			}
			gp.ParticipationStatus = game.ParticipationSkipped
		}
		return e.advanceToRoundResults(agg, r, now), nil

	default:
		return nil, ErrInvalidPhase
	}
}

// --- LEAVE_ROOM / KICK_PLAYER ---

func (e *Engine) leaveRoom(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	p := agg.PlayerByID(cmd.PlayerID)
	if p == nil {
		return nil, ErrPlayerNotFound
	}
	if p.LeftAt != nil {
		return nil, ErrPlayerAlreadyLeft
	}
	events, err := e.applyLeave(agg, p, cmd.Now)
	if err != nil {
		return nil, err
	}
	return append([]Event{{Type: EventPlayerLeft, Revision: agg.Room.Revision, Data: map[string]any{"playerId": p.ID}}}, events...), nil
}

func (e *Engine) kickPlayer(ctx context.Context, agg *Aggregate, cmd Command) ([]Event, error) {
	if !cmd.IsAdmin {
		return nil, ErrNotAllowed
	}
	playerID, _ := cmd.Payload["playerId"].(string)
	p := agg.PlayerByID(playerID)
	if p == nil {
		return nil, ErrPlayerNotFound
	}
	if p.LeftAt != nil {
		return nil, ErrPlayerAlreadyLeft
	}
	events, err := e.applyLeave(agg, p, cmd.Now)
	if err != nil {
		return nil, err
	}
	return append([]Event{{Type: EventPlayerKicked, Revision: agg.Room.Revision, Data: map[string]any{"playerId": p.ID}}}, events...), nil
}

// applyLeave applies the leave rules for the current phase and returns the
// resulting transition events (excluding the PLAYER_LEFT/PLAYER_KICKED event).
func (e *Engine) applyLeave(agg *Aggregate, p *player.Player, now time.Time) ([]Event, error) {
	wasHost := p.Role == player.RoleHost
	p.LeftAt = &now
	p.Connected = false

	var events []Event

	if agg.Game == nil {
		// Before game start (LOBBY): remove player from the room entirely.
		p.Role = player.RolePlayer
		if wasHost {
			events = append(events, e.transferHost(agg, p.ID)...)
		}
		return events, nil
	}

	gp := agg.GamePlayerByPlayerID(p.ID)
	switch agg.Phase {
	case game.PhasePreparation:
		if gp != nil {
			gp.ParticipationStatus = game.ParticipationLeft
			removePreparedTurn(agg, gp.ID)
			if allActivePrepared(agg) {
				evs, err := e.advanceToRoundSelection(agg, now)
				if err != nil {
					return nil, err
				}
				events = append(events, evs...)
			}
		}

	case game.PhaseRoundSelection:
		if gp != nil {
			r := agg.CurrentRound()
			if r != nil && r.ActiveGamePlayerID == gp.ID {
				// Active player leaves: cancel the current round and move on.
				r.Status = "CANCELLED"
				r.FinishedAt = &now
				gp.ParticipationStatus = game.ParticipationLeft
				next := e.nextEligibleActivePlayer(agg)
				if next != nil {
					nr, err := e.startRound(agg, next, now)
					if err != nil {
						return nil, err
					}
					events = append(events,
						Event{Type: EventRoundStarted, Revision: agg.Room.Revision, Data: map[string]any{"roundId": nr.ID, "activeGamePlayerId": next.ID}},
						Event{Type: EventPhaseChanged, Revision: agg.Room.Revision, Data: map[string]any{"phase": string(agg.Phase)}},
					)
				} else {
					events = append(events, e.finishCycle(agg, now)...)
				}
			} else if r != nil {
				gp.ParticipationStatus = game.ParticipationLeft
				removeSubmission(agg, r.ID, gp.ID)
				if allNonActiveSubmitted(agg, r) {
					events = append(events, e.advanceToRoundVoting(agg, r, now)...)
				}
			} else {
				gp.ParticipationStatus = game.ParticipationLeft
			}
		}

	case game.PhaseRoundVoting:
		if gp != nil {
			gp.ParticipationStatus = game.ParticipationLeft
			r := agg.CurrentRound()
			if r != nil && allNonActiveVoted(agg, r) {
				events = append(events, e.advanceToRoundResults(agg, r, now)...)
			}
		}

	case game.PhaseRoundResults:
		if gp != nil {
			gp.ParticipationStatus = game.ParticipationLeft
		}
	}

	if wasHost {
		events = append(events, e.transferHost(agg, p.ID)...)
	}
	return events, nil
}

// transferHost promotes the oldest active joined player (excluding
// excludePlayerID) to HOST and demotes the current host to PLAYER.
func (e *Engine) transferHost(agg *Aggregate, excludePlayerID string) []Event {
	var newHost *player.Player
	for _, p := range agg.Players {
		if p.ID == excludePlayerID {
			continue
		}
		if p.LeftAt != nil {
			continue
		}
		if newHost == nil || p.JoinedAt.Before(newHost.JoinedAt) {
			newHost = p
		}
	}
	if newHost == nil {
		return nil
	}
	for _, p := range agg.Players {
		if p.Role == player.RoleHost {
			p.Role = player.RolePlayer
		}
	}
	newHost.Role = player.RoleHost
	return []Event{{Type: EventHostTransferred, Revision: agg.Room.Revision, Data: map[string]any{"newHostPlayerId": newHost.ID}}}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
