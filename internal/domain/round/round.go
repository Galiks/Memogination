// Package round defines the round, turn, submission, vote, and hand domain
// models.
package round

import "time"

// Round is a single active-player turn within a cycle.
type Round struct {
	ID                 string     `json:"id"`
	GameID             string     `json:"gameId"`
	CycleID            string     `json:"cycleId"`
	ActiveGamePlayerID string     `json:"activeGamePlayerId"`
	Phase              string     `json:"phase"`
	SituationText      string     `json:"situationText"`
	OriginalMemeID     string     `json:"originalMemeId"`
	Status             string     `json:"status"`
	StartedAt          time.Time  `json:"startedAt"`
	DeadlineAt         *time.Time `json:"deadlineAt"`
	FinishedAt         *time.Time `json:"finishedAt"`
}

// PreparedTurn is a player's prepared situation/meme for a cycle.
type PreparedTurn struct {
	ID             string    `json:"id"`
	CycleID        string    `json:"cycleId"`
	GamePlayerID   string    `json:"gamePlayerId"`
	SituationText  string    `json:"situationText"`
	OriginalMemeID string    `json:"originalMemeId"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// RoundSubmission is a player's submitted meme for a round.
type RoundSubmission struct {
	ID           string    `json:"id"`
	RoundID      string    `json:"roundId"`
	GamePlayerID string    `json:"gamePlayerId"`
	MemeID       string    `json:"memeId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// VoteOption is a candidate meme that players can vote for in a round.
type VoteOption struct {
	ID                string `json:"id"`
	RoundID           string `json:"roundId"`
	Number            int    `json:"number"`
	MemeID            string `json:"memeId"`
	OwnerGamePlayerID string `json:"ownerGamePlayerId"`
	IsOriginal        bool   `json:"isOriginal"`
}

// Vote is a single player's vote in a round.
type Vote struct {
	ID           string    `json:"id"`
	RoundID      string    `json:"roundId"`
	GamePlayerID string    `json:"gamePlayerId"`
	VoteOptionID string    `json:"voteOptionId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// RoundScore records the score change for a player in a round.
type RoundScore struct {
	RoundID       string `json:"roundId"`
	GamePlayerID  string `json:"gamePlayerId"`
	PreviousScore int    `json:"previousScore"`
	Delta         int    `json:"delta"`
	NewScore      int    `json:"newScore"`
}

// Hand is the set of meme IDs dealt to a game player.
type Hand struct {
	GamePlayerID string   `json:"gamePlayerId"`
	MemeIDs      []string `json:"memeIds"`
	// Kind distinguishes preparation hands from per-round hands
	// (e.g. "PREPARATION" or "ROUND").
	Kind string `json:"kind"`
	// RoundID is set for per-round hands and empty for preparation hands.
	RoundID string `json:"roundId"`
}
