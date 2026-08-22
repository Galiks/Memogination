package engine

import (
	"math/rand"
	"sort"

	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/round"
)

// MemeDealer deals hands and picks random content. It is the only component
// that uses math/rand, enabling deterministic tests via a fixed seed.
type MemeDealer struct {
	rng *rand.Rand
	// lastUsed tracks recency of meme usage (higher = more recently used).
	lastUsed map[string]int
	// usedCounter is a monotonically increasing counter for recency ordering.
	usedCounter int
}

// NewMemeDealer returns a MemeDealer seeded with the given seed.
func NewMemeDealer(seed int64) *MemeDealer {
	return &MemeDealer{
		rng:      rand.New(rand.NewSource(seed)),
		lastUsed: make(map[string]int),
	}
}

// Shuffle performs an in-place Fisher-Yates shuffle using the dealer's RNG.
// It is a package-level generic function because Go does not support generic
// methods.
func Shuffle[T any](rng *rand.Rand, slice []T) {
	rng.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

// RandomSituation returns the text of a random enabled situation.
func (d *MemeDealer) RandomSituation(agg *Aggregate) string {
	sits := agg.EnabledSituations()
	if len(sits) == 0 {
		return ""
	}
	return sits[d.rng.Intn(len(sits))].Text
}

// RandomMemeFromHand returns a random meme ID from a hand, or "" if empty.
func (d *MemeDealer) RandomMemeFromHand(hand *round.Hand) string {
	if hand == nil || len(hand.MemeIDs) == 0 {
		return ""
	}
	return hand.MemeIDs[d.rng.Intn(len(hand.MemeIDs))]
}

// markUsed records that the given memes were just used.
func (d *MemeDealer) markUsed(memeIDs []string) {
	for _, id := range memeIDs {
		d.usedCounter++
		d.lastUsed[id] = d.usedCounter
	}
}

// sortByRecency orders the pool so least-recently-used memes come first.
// Memes never used (recency 0) come first. The sort is stable so a prior
// shuffle is preserved within equal-recency groups.
func (d *MemeDealer) sortByRecency(pool []string) {
	sort.SliceStable(pool, func(i, j int) bool {
		return d.lastUsed[pool[i]] < d.lastUsed[pool[j]]
	})
}

// DealPreparationHands assigns each ACTIVE game player a disjoint hand of
// HandSize unique enabled memes. All hands within the cycle are disjoint,
// guaranteeing unique Original Memes.
func (d *MemeDealer) DealPreparationHands(agg *Aggregate) error {
	players := agg.ActiveGamePlayers()
	handSize := agg.Settings.HandSize
	needed := len(players) * handSize

	pool := agg.EnabledMemes()
	if len(pool) < needed {
		return ErrNotEnoughMemes
	}

	Shuffle(d.rng, pool)
	d.sortByRecency(pool)

	// Replace existing preparation hands.
	kept := agg.Hands[:0]
	for _, h := range agg.Hands {
		if h.Kind != HandKindPreparation {
			kept = append(kept, h)
		}
	}
	agg.Hands = kept

	idx := 0
	for _, gp := range players {
		hand := append([]string(nil), pool[idx:idx+handSize]...)
		idx += handSize
		agg.Hands = append(agg.Hands, &round.Hand{
			GamePlayerID: gp.ID,
			MemeIDs:      hand,
			Kind:         HandKindPreparation,
		})
		d.markUsed(hand)
	}
	return nil
}

// DealRoundHands assigns each non-active participating (ACTIVE) game player a
// disjoint hand of HandSize unique enabled memes, excluding all Original Memes
// of the current cycle and preferring memes not recently shown.
func (d *MemeDealer) DealRoundHands(agg *Aggregate, r *round.Round) error {
	// Exclude original memes of the current cycle.
	excluded := map[string]bool{}
	if agg.Cycle != nil {
		for _, pt := range agg.PreparedTurns {
			if pt.CycleID == agg.Cycle.ID && pt.OriginalMemeID != "" {
				excluded[pt.OriginalMemeID] = true
			}
		}
	}

	pool := make([]string, 0, len(agg.Memes))
	for _, m := range agg.Memes {
		if m.Enabled && !excluded[m.ID] {
			pool = append(pool, m.ID)
		}
	}

	players := make([]*game.GamePlayer, 0)
	for _, gp := range agg.ActiveGamePlayers() {
		if gp.ID == r.ActiveGamePlayerID {
			continue
		}
		players = append(players, gp)
	}

	handSize := agg.Settings.HandSize
	needed := len(players) * handSize
	if len(pool) < needed {
		return ErrNotEnoughMemes
	}

	Shuffle(d.rng, pool)
	d.sortByRecency(pool)

	// Replace existing round hands for this round.
	kept := agg.Hands[:0]
	for _, h := range agg.Hands {
		if h.Kind == HandKindRound && h.RoundID == r.ID {
			continue
		}
		kept = append(kept, h)
	}
	agg.Hands = kept

	idx := 0
	for _, gp := range players {
		hand := append([]string(nil), pool[idx:idx+handSize]...)
		idx += handSize
		agg.Hands = append(agg.Hands, &round.Hand{
			GamePlayerID: gp.ID,
			MemeIDs:      hand,
			Kind:         HandKindRound,
			RoundID:      r.ID,
		})
		d.markUsed(hand)
	}
	return nil
}
