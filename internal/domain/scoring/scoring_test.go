package scoring

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deltasByPlayer(ds []ScoreDelta) map[string]int {
	m := make(map[string]int, len(ds))
	for _, d := range ds {
		m[d.GamePlayerID] = d.Delta
	}
	return m
}

func TestCalculateRoundAllGuessed(t *testing.T) {
	cfg := DefaultScoreConfig()
	facts := RoundFacts{
		ActivePlayerID:   "active",
		TotalVotes:       2,
		VotesForOriginal: 2,
		GuesserIDs:       []string{"g1", "g2"},
	}
	got := deltasByPlayer((ScoreCalculator{}).CalculateRound(facts, cfg))
	assert.Equal(t, -3, got["active"])
	assert.Equal(t, 0, got["g1"])
	assert.Equal(t, 0, got["g2"])
}

func TestCalculateRoundNoneGuessed(t *testing.T) {
	cfg := DefaultScoreConfig()
	facts := RoundFacts{
		ActivePlayerID:   "active",
		TotalVotes:       2,
		VotesForOriginal: 0,
		GuesserIDs:       []string{},
	}
	got := deltasByPlayer((ScoreCalculator{}).CalculateRound(facts, cfg))
	assert.Equal(t, -2, got["active"])
}

func TestCalculateRoundPartial(t *testing.T) {
	cfg := DefaultScoreConfig()
	facts := RoundFacts{
		ActivePlayerID:   "active",
		TotalVotes:       3,
		VotesForOriginal: 2,
		GuesserIDs:       []string{"g1", "g2"},
	}
	got := deltasByPlayer((ScoreCalculator{}).CalculateRound(facts, cfg))
	// active = base(3) + perGuesser(1)*2 = 5
	assert.Equal(t, 5, got["active"])
	assert.Equal(t, 3, got["g1"])
	assert.Equal(t, 3, got["g2"])
}

func TestCalculateRoundVoteForSubmittedMeme(t *testing.T) {
	cfg := DefaultScoreConfig()
	facts := RoundFacts{
		ActivePlayerID:   "active",
		TotalVotes:       2,
		VotesForOriginal: 1,
		GuesserIDs:       []string{"g1"},
		VoteForSubmittedMeme: map[string]bool{
			"g1": true,
			"g2": true,
		},
	}
	got := deltasByPlayer((ScoreCalculator{}).CalculateRound(facts, cfg))
	// active = 3 + 1*1 = 4
	assert.Equal(t, 4, got["active"])
	// g1 guessed (3) + voted for own submitted (1) = 4
	assert.Equal(t, 4, got["g1"])
	// g2 only voted for own submitted (1)
	assert.Equal(t, 1, got["g2"])
}

func TestDefaultScoreConfig(t *testing.T) {
	cfg := DefaultScoreConfig()
	require.Equal(t, -3, cfg.AllGuessedActivePlayer)
	require.Equal(t, 0, cfg.AllGuessedGuesser)
	require.Equal(t, -2, cfg.NoneGuessedActivePlayer)
	require.Equal(t, 0, cfg.NoneGuessedOtherPlayer)
	require.Equal(t, 3, cfg.PartialActiveBase)
	require.Equal(t, 1, cfg.PartialActivePerGuesser)
	require.Equal(t, 3, cfg.PartialGuesser)
	require.Equal(t, 1, cfg.VoteForSubmittedMeme)
}
