// Package scoring defines the pure, deterministic scoring rules for rounds.
package scoring

// ScoreConfig holds the scoring weights for a round.
type ScoreConfig struct {
	AllGuessedActivePlayer  int `json:"allGuessedActivePlayer"`
	AllGuessedGuesser       int `json:"allGuessedGuesser"`
	NoneGuessedActivePlayer int `json:"noneGuessedActivePlayer"`
	NoneGuessedOtherPlayer  int `json:"noneGuessedOtherPlayer"`
	PartialActiveBase       int `json:"partialActiveBase"`
	PartialActivePerGuesser int `json:"partialActivePerGuesser"`
	PartialGuesser          int `json:"partialGuesser"`
	VoteForSubmittedMeme    int `json:"voteForSubmittedMeme"`
}

// DefaultScoreConfig returns the Standard scoring preset.
func DefaultScoreConfig() ScoreConfig {
	return ScoreConfig{
		AllGuessedActivePlayer:  -3,
		AllGuessedGuesser:       0,
		NoneGuessedActivePlayer: -2,
		NoneGuessedOtherPlayer:  0,
		PartialActiveBase:       3,
		PartialActivePerGuesser: 1,
		PartialGuesser:          3,
		VoteForSubmittedMeme:    1,
	}
}

// Preset identifies a named scoring configuration.
type Preset string

const (
	PresetStandard Preset = "Standard"
	PresetCustom   Preset = "Custom"
)

// RoundFacts describes the outcome of a round for scoring.
type RoundFacts struct {
	ActivePlayerID       string
	OriginalOwnerID      string
	TotalVotes           int
	VotesForOriginal     int
	GuesserIDs           []string
	VoteForSubmittedMeme map[string]bool
}

// ScoreDelta is a single player's score change for a round.
type ScoreDelta struct {
	GamePlayerID string
	Delta        int
}

// ScoreCalculator computes round score deltas. It is pure and deterministic.
type ScoreCalculator struct{}

// CalculateRound computes the score deltas for a round given its facts.
//
// Rules:
//   - If every non-active participating player voted for the original, the
//     active player receives AllGuessedActivePlayer and each guesser receives
//     AllGuessedGuesser.
//   - If nobody voted for the original, the active player receives
//     NoneGuessedActivePlayer and each non-active participating player receives
//     NoneGuessedOtherPlayer.
//   - Otherwise (partial), the active player receives
//     PartialActiveBase + PartialActivePerGuesser*numGuessed and each guesser
//     who guessed the original receives PartialGuesser.
//   - Additionally, each player who voted for their own submitted meme receives
//     VoteForSubmittedMeme.
//
// The active player never votes.
func (ScoreCalculator) CalculateRound(facts RoundFacts, cfg ScoreConfig) []ScoreDelta {
	deltas := make(map[string]int)
	add := func(id string, delta int) {
		if id == "" {
			return
		}
		deltas[id] += delta
	}

	// The active player never votes, so the participating non-active players are
	// exactly the guessers (those who voted).
	numGuessed := facts.VotesForOriginal
	participants := facts.GuesserIDs

	switch {
	case facts.TotalVotes > 0 && numGuessed == facts.TotalVotes:
		// All participating players guessed the original.
		add(facts.ActivePlayerID, cfg.AllGuessedActivePlayer)
		for _, id := range participants {
			add(id, cfg.AllGuessedGuesser)
		}
	case numGuessed == 0:
		// Nobody guessed the original.
		add(facts.ActivePlayerID, cfg.NoneGuessedActivePlayer)
		for _, id := range participants {
			add(id, cfg.NoneGuessedOtherPlayer)
		}
	default:
		// Partial: some guessed the original.
		add(facts.ActivePlayerID, cfg.PartialActiveBase+cfg.PartialActivePerGuesser*numGuessed)
		for _, id := range participants {
			add(id, cfg.PartialGuesser)
		}
	}

	// Bonus for voting for one's own submitted meme.
	for id, voted := range facts.VoteForSubmittedMeme {
		if voted {
			add(id, cfg.VoteForSubmittedMeme)
		}
	}

	out := make([]ScoreDelta, 0, len(deltas))
	for id, delta := range deltas {
		out = append(out, ScoreDelta{GamePlayerID: id, Delta: delta})
	}
	return out
}
