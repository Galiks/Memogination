package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/player"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
)

const testSeed = 42

func now() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

// newTestEngine returns an engine with a fixed-seed dealer.
func newTestEngine() *Engine {
	return NewWithDealer(NewMemeDealer(testSeed))
}

// newTestAggregate builds a valid aggregate with n players, enough memes and
// situations. The first player is the HOST.
func newTestAggregate(n int) *Aggregate {
	settings := room.DefaultRoomSettings()
	settings.MinPlayers = 2
	settings.HandSize = 3

	agg := &Aggregate{
		Room:     &room.Room{ID: "room-1", Code: "ABC123", Revision: 0, State: room.StateLobby, CreatedAt: now()},
		Settings: settings,
	}

	// Enough memes: requiredMemes = max(N*H, N+(N-1)*H).
	required := n * settings.HandSize
	if r := n + (n-1)*settings.HandSize; r > required {
		required = r
	}
	for i := 0; i < required+5; i++ {
		agg.Memes = append(agg.Memes, &content.Meme{ID: "meme-" + itoa(i), Enabled: true})
	}
	agg.Situations = append(agg.Situations, &content.Situation{ID: "sit-1", Text: "Situation one", Enabled: true})
	agg.Situations = append(agg.Situations, &content.Situation{ID: "sit-2", Text: "Situation two", Enabled: true})

	for i := 0; i < n; i++ {
		role := player.RolePlayer
		if i == 0 {
			role = player.RoleHost
		}
		agg.Players = append(agg.Players, &player.Player{
			ID:        "player-" + itoa(i),
			RoomID:    agg.Room.ID,
			Name:      "Player " + itoa(i),
			Role:      role,
			Connected: true,
			JoinedAt:  now().Add(time.Duration(i) * time.Minute),
		})
	}
	return agg
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func cmd(t string, playerID string, payload map[string]any) Command {
	return Command{Type: t, PlayerID: playerID, Payload: payload, Now: now()}
}

func startGame(t *testing.T, agg *Aggregate) {
	t.Helper()
	_, err := newTestEngine().Handle(context.Background(), agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhasePreparation, agg.Phase)
}

// prepareAll submits a valid preparation for every active player.
func prepareAll(t *testing.T, agg *Aggregate) {
	t.Helper()
	e := newTestEngine()
	for _, gp := range agg.ActiveGamePlayers() {
		hand := agg.HandFor(gp.ID, HandKindPreparation)
		require.NotNil(t, hand)
		_, err := e.Handle(context.Background(), agg, cmd(CommandSubmitPreparation, gp.PlayerID, map[string]any{
			"situationText": "Situation one",
			"memeId":        hand.MemeIDs[0],
		}))
		require.NoError(t, err)
	}
	require.Equal(t, game.PhaseRoundSelection, agg.Phase)
}

// submitAll submits a valid meme for every non-active player of the current round.
func submitAll(t *testing.T, agg *Aggregate) {
	t.Helper()
	e := newTestEngine()
	r := agg.CurrentRound()
	require.NotNil(t, r)
	for _, gp := range agg.ActiveGamePlayers() {
		if gp.ID == r.ActiveGamePlayerID {
			continue
		}
		hand := agg.HandFor(gp.ID, HandKindRound)
		require.NotNil(t, hand)
		_, err := e.Handle(context.Background(), agg, cmd(CommandSubmitRoundMeme, gp.PlayerID, map[string]any{"memeId": hand.MemeIDs[0]}))
		require.NoError(t, err)
	}
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)
}

// voteAll makes every non-active player vote for the original meme.
func voteAll(t *testing.T, agg *Aggregate) {
	t.Helper()
	e := newTestEngine()
	r := agg.CurrentRound()
	require.NotNil(t, r)
	var original *round.VoteOption
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID && o.IsOriginal {
			original = o
			break
		}
	}
	require.NotNil(t, original)
	for _, gp := range agg.ActiveGamePlayers() {
		if gp.ID == r.ActiveGamePlayerID {
			continue
		}
		_, err := e.Handle(context.Background(), agg, cmd(CommandSubmitVote, gp.PlayerID, map[string]any{"voteOptionId": original.ID}))
		require.NoError(t, err)
	}
	require.Equal(t, game.PhaseRoundResults, agg.Phase)
}

func TestStartGameHappyPath(t *testing.T) {
	agg := newTestAggregate(4)
	e := newTestEngine()
	events, err := e.Handle(context.Background(), agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)

	require.Equal(t, room.StateInGame, agg.Room.State)
	// The engine no longer increments the room revision internally; the
	// coordinator owns the single revision increment per command.
	require.Equal(t, 0, agg.Room.Revision)
	require.NotNil(t, agg.Game)
	require.Equal(t, game.StateActive, agg.Game.State)
	require.NotNil(t, agg.Cycle)
	require.Equal(t, 1, agg.Cycle.Number)
	require.Equal(t, game.PhasePreparation, agg.Phase)
	require.Len(t, agg.GamePlayers, 4)
	require.Len(t, agg.PreparedTurns, 0)

	// Hands: 4 players * 3 = 12 disjoint memes.
	require.Len(t, agg.Hands, 4)
	seen := map[string]bool{}
	for _, h := range agg.Hands {
		require.Equal(t, HandKindPreparation, h.Kind)
		require.Len(t, h.MemeIDs, 3)
		for _, m := range h.MemeIDs {
			require.False(t, seen[m], "hand must be disjoint")
			seen[m] = true
		}
	}

	types := []string{}
	for _, ev := range events {
		types = append(types, ev.Type)
	}
	assert.Equal(t, []string{EventGameStarted, EventPhaseChanged}, types)
}

func TestStartGameNotEnoughMemes(t *testing.T) {
	agg := newTestAggregate(4)
	// Remove memes so fewer than required are available.
	agg.Memes = agg.Memes[:5]
	_, err := newTestEngine().Handle(context.Background(), agg, cmd(CommandStartGame, "player-0", nil))
	require.ErrorIs(t, err, ErrNotEnoughMemes)
	assert.Equal(t, "NOT_ENOUGH_MEMES", Code(err))
}

func TestStartGameNotEnoughPlayers(t *testing.T) {
	agg := newTestAggregate(1)
	_, err := newTestEngine().Handle(context.Background(), agg, cmd(CommandStartGame, "player-0", nil))
	require.ErrorIs(t, err, ErrInvalidCommand)
}

func TestStartGameNonHostNotAllowed(t *testing.T) {
	agg := newTestAggregate(4)
	_, err := newTestEngine().Handle(context.Background(), agg, cmd(CommandStartGame, "player-1", nil))
	require.ErrorIs(t, err, ErrNotAllowed)
}

func TestFullFlowFourPlayers(t *testing.T) {
	agg := newTestAggregate(4)
	e := newTestEngine()
	ctx := context.Background()

	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)

	prepareAll(t, agg)
	require.Equal(t, game.PhaseRoundSelection, agg.Phase)

	// Round 1.
	submitAll(t, agg)
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)
	voteAll(t, agg)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)
	require.Len(t, agg.RoundScores, 4)

	// Rounds 2..4.
	for i := 0; i < 3; i++ {
		_, err := e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundSelection, agg.Phase)
		submitAll(t, agg)
		voteAll(t, agg)
		require.Equal(t, game.PhaseRoundResults, agg.Phase)
	}

	// After 4 rounds all players have been active; NEXT_ROUND finishes the game.
	_, err = e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseGameResults, agg.Phase)
	require.Equal(t, game.StateFinished, agg.Game.State)
	require.Equal(t, room.StateClosed, agg.Room.State)
	require.Len(t, agg.Rounds, 4)

	// Scores were computed for every round (4 rounds x 4 players).
	require.Len(t, agg.RoundScores, 16)
	for _, gp := range agg.GamePlayers {
		require.NotZero(t, gp.Score)
	}
}

func TestSubmitPreparationDuplicateRejected(t *testing.T) {
	agg := newTestAggregate(2)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)

	gp := agg.GamePlayers[0]
	hand := agg.HandFor(gp.ID, HandKindPreparation)
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitPreparation, gp.PlayerID, map[string]any{"situationText": "Situation one", "memeId": hand.MemeIDs[0]}))
	require.NoError(t, err)
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitPreparation, gp.PlayerID, map[string]any{"situationText": "Situation two", "memeId": hand.MemeIDs[1]}))
	require.ErrorIs(t, err, ErrCommandAlreadyProcessed)
}

func TestSubmitVoteDuplicateRejected(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)

	r := agg.CurrentRound()
	var original *round.VoteOption
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID && o.IsOriginal {
			original = o
			break
		}
	}
	voter := agg.GamePlayers[1]
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitVote, voter.PlayerID, map[string]any{"voteOptionId": original.ID}))
	require.NoError(t, err)
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitVote, voter.PlayerID, map[string]any{"voteOptionId": original.ID}))
	require.ErrorIs(t, err, ErrCommandAlreadyProcessed)
}

func TestSubmitVoteOwnMemeForbidden(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)

	// Find the vote option owned by player-1 and try to vote for it.
	r := agg.CurrentRound()
	voter := agg.GamePlayers[1]
	var own *round.VoteOption
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID && o.OwnerGamePlayerID == voter.ID {
			own = o
			break
		}
	}
	require.NotNil(t, own)
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitVote, voter.PlayerID, map[string]any{"voteOptionId": own.ID}))
	require.ErrorIs(t, err, ErrOwnMemeVoteForbidden)
}

func TestActivePlayerCannotVote(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)

	r := agg.CurrentRound()
	var original *round.VoteOption
	for _, o := range agg.VoteOptions {
		if o.RoundID == r.ID && o.IsOriginal {
			original = o
			break
		}
	}
	active := agg.GamePlayerByID(r.ActiveGamePlayerID)
	_, err = e.Handle(ctx, agg, cmd(CommandSubmitVote, active.PlayerID, map[string]any{"voteOptionId": original.ID}))
	require.ErrorIs(t, err, ErrNotAllowed)
}

func TestForceResolvePhase(t *testing.T) {
	ctx := context.Background()

	t.Run("preparation", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		events, err := e.Handle(ctx, agg, cmd(CommandForceResolvePhase, "player-0", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundSelection, agg.Phase)
		require.Len(t, agg.PreparedTurns, 3)
		assert.Equal(t, EventForceResolved, events[0].Type)
	})

	t.Run("round_selection", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		prepareAll(t, agg)
		_, err = e.Handle(ctx, agg, cmd(CommandForceResolvePhase, "player-0", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundVoting, agg.Phase)
		require.Len(t, agg.Submissions, 2)
	})

	t.Run("round_voting", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		prepareAll(t, agg)
		submitAll(t, agg)
		_, err = e.Handle(ctx, agg, cmd(CommandForceResolvePhase, "player-0", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundResults, agg.Phase)
		// Both non-active players were skipped (no votes), so only the active
		// player receives a score delta.
		require.Len(t, agg.RoundScores, 1)
	})
}

func TestSkippedVoterCanParticipateNextRound(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)

	// Timeout the voting phase: non-voters are marked SKIPPED.
	_, err = e.Handle(ctx, agg, cmd(CommandTimeoutPhase, "", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)
	r := agg.CurrentRound()
	for _, gp := range agg.GamePlayers {
		if gp.ID == r.ActiveGamePlayerID {
			require.Equal(t, game.ParticipationActive, gp.ParticipationStatus)
		} else {
			require.Equal(t, game.ParticipationSkipped, gp.ParticipationStatus)
		}
	}

	// Advance to the next round: SKIPPED must be reset to ACTIVE.
	_, err = e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundSelection, agg.Phase)
	for _, gp := range agg.GamePlayers {
		require.Equal(t, game.ParticipationActive, gp.ParticipationStatus)
	}

	// The previously-skipped players can submit and vote again.
	submitAll(t, agg)
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)
	voteAll(t, agg)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)
}

func TestSkippedVoterCanParticipateNextCycle(t *testing.T) {
	agg := newTestAggregate(3)
	agg.Settings.InfiniteGame = true
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)

	// Timeout voting to skip the non-voters.
	_, err = e.Handle(ctx, agg, cmd(CommandTimeoutPhase, "", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)

	// Finish the remaining rounds of the cycle.
	for i := 0; i < 2; i++ {
		_, err := e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
		require.NoError(t, err)
		submitAll(t, agg)
		voteAll(t, agg)
	}
	_, err = e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseCycleResults, agg.Phase)

	// Start the next cycle: SKIPPED must be reset to ACTIVE.
	_, err = e.Handle(ctx, agg, cmd(CommandStartNextCycle, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhasePreparation, agg.Phase)
	for _, gp := range agg.GamePlayers {
		require.Equal(t, game.ParticipationActive, gp.ParticipationStatus)
	}
}

func TestTimeoutPhase(t *testing.T) {
	ctx := context.Background()

	t.Run("preparation", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		events, err := e.Handle(ctx, agg, cmd(CommandTimeoutPhase, "", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundSelection, agg.Phase)
		require.Len(t, agg.PreparedTurns, 3)
		assert.Equal(t, EventTimeoutApplied, events[0].Type)
	})

	t.Run("round_selection", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		prepareAll(t, agg)
		_, err = e.Handle(ctx, agg, cmd(CommandTimeoutPhase, "", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundVoting, agg.Phase)
		require.Len(t, agg.Submissions, 2)
	})

	t.Run("round_voting", func(t *testing.T) {
		agg := newTestAggregate(3)
		e := newTestEngine()
		_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
		require.NoError(t, err)
		prepareAll(t, agg)
		submitAll(t, agg)
		_, err = e.Handle(ctx, agg, cmd(CommandTimeoutPhase, "", nil))
		require.NoError(t, err)
		require.Equal(t, game.PhaseRoundResults, agg.Phase)
		require.Len(t, agg.RoundScores, 1)
	})
}

func TestLeaveRoomPreparation(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)

	// player-1 leaves during preparation.
	_, err = e.Handle(ctx, agg, cmd(CommandLeaveRoom, "player-1", nil))
	require.NoError(t, err)
	gp := agg.GamePlayerByPlayerID("player-1")
	require.Equal(t, game.ParticipationLeft, gp.ParticipationStatus)
	require.Nil(t, agg.PreparedTurnFor(gp.ID))
	// Still in preparation (not all remaining prepared).
	require.Equal(t, game.PhasePreparation, agg.Phase)
}

func TestLeaveRoomSelectionNonActive(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)

	// player-1 (non-active) leaves during round selection.
	_, err = e.Handle(ctx, agg, cmd(CommandLeaveRoom, "player-1", nil))
	require.NoError(t, err)
	gp := agg.GamePlayerByPlayerID("player-1")
	require.Equal(t, game.ParticipationLeft, gp.ParticipationStatus)
	// Remaining non-active player (player-2) still needs to submit.
	require.Equal(t, game.PhaseRoundSelection, agg.Phase)
}

func TestLeaveRoomSelectionActive(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)

	// The active player (player-0) leaves during round selection.
	r := agg.CurrentRound()
	active := agg.GamePlayerByID(r.ActiveGamePlayerID)
	_, err = e.Handle(ctx, agg, cmd(CommandLeaveRoom, active.PlayerID, nil))
	require.NoError(t, err)
	require.Equal(t, game.ParticipationLeft, active.ParticipationStatus)
	// A new round starts for the next eligible player.
	require.Equal(t, game.PhaseRoundSelection, agg.Phase)
	require.NotEqual(t, r.ID, agg.CurrentRound().ID)
}

func TestLeaveRoomVoting(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)

	// player-1 leaves during voting.
	_, err = e.Handle(ctx, agg, cmd(CommandLeaveRoom, "player-1", nil))
	require.NoError(t, err)
	gp := agg.GamePlayerByPlayerID("player-1")
	require.Equal(t, game.ParticipationLeft, gp.ParticipationStatus)
	// player-2 still needs to vote.
	require.Equal(t, game.PhaseRoundVoting, agg.Phase)
}

func TestLeaveRoomResults(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)
	voteAll(t, agg)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)

	// player-1 leaves during results.
	_, err = e.Handle(ctx, agg, cmd(CommandLeaveRoom, "player-1", nil))
	require.NoError(t, err)
	gp := agg.GamePlayerByPlayerID("player-1")
	require.Equal(t, game.ParticipationLeft, gp.ParticipationStatus)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)
}

func TestInfiniteModeStartNextCycle(t *testing.T) {
	agg := newTestAggregate(3)
	agg.Settings.InfiniteGame = true
	e := newTestEngine()
	ctx := context.Background()
	_, err := e.Handle(ctx, agg, cmd(CommandStartGame, "player-0", nil))
	require.NoError(t, err)
	prepareAll(t, agg)
	submitAll(t, agg)
	voteAll(t, agg)
	require.Equal(t, game.PhaseRoundResults, agg.Phase)

	// Finish all rounds of the cycle.
	for i := 0; i < 2; i++ {
		_, err := e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
		require.NoError(t, err)
		submitAll(t, agg)
		voteAll(t, agg)
	}
	_, err = e.Handle(ctx, agg, cmd(CommandNextRound, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhaseCycleResults, agg.Phase)

	// Capture scores before the next cycle.
	scoresBefore := map[string]int{}
	for _, gp := range agg.GamePlayers {
		scoresBefore[gp.ID] = gp.Score
	}

	_, err = e.Handle(ctx, agg, cmd(CommandStartNextCycle, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, game.PhasePreparation, agg.Phase)
	require.Equal(t, 2, agg.Cycle.Number)
	require.Len(t, agg.PreparedTurns, 0)
	require.Len(t, agg.Rounds, 0)

	// Scores preserved.
	for _, gp := range agg.GamePlayers {
		require.Equal(t, scoresBefore[gp.ID], gp.Score)
	}
}

func TestHostTransferOnLeave(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()

	// Host (player-0) leaves in LOBBY.
	events, err := e.Handle(ctx, agg, cmd(CommandLeaveRoom, "player-0", nil))
	require.NoError(t, err)
	require.Equal(t, player.RolePlayer, agg.PlayerByID("player-0").Role)
	require.Equal(t, player.RoleHost, agg.PlayerByID("player-1").Role)

	found := false
	for _, ev := range events {
		if ev.Type == EventHostTransferred {
			found = true
			require.Equal(t, "player-1", ev.Data["newHostPlayerId"])
		}
	}
	assert.True(t, found)
}

func TestKickPlayerAdminOnly(t *testing.T) {
	agg := newTestAggregate(3)
	e := newTestEngine()
	ctx := context.Background()

	// Non-admin cannot kick.
	_, err := e.Handle(ctx, agg, cmd(CommandKickPlayer, "player-0", map[string]any{"playerId": "player-1"}))
	require.ErrorIs(t, err, ErrNotAllowed)

	// Admin can kick.
	events, err := e.Handle(ctx, agg, Command{Type: CommandKickPlayer, IsAdmin: true, Payload: map[string]any{"playerId": "player-1"}, Now: now()})
	require.NoError(t, err)
	require.Equal(t, player.RolePlayer, agg.PlayerByID("player-1").Role)
	require.NotNil(t, agg.PlayerByID("player-1").LeftAt)
	assert.Equal(t, EventPlayerKicked, events[0].Type)
}
