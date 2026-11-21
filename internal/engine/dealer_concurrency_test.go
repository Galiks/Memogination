package engine

import (
	"sync"
	"testing"

	"github.com/memomarium/memomarium/internal/domain/content"
	"github.com/memomarium/memomarium/internal/domain/game"
	"github.com/memomarium/memomarium/internal/domain/room"
	"github.com/memomarium/memomarium/internal/domain/round"
	"github.com/stretchr/testify/require"
)

func TestMemeDealerSeededShuffleIsDeterministic(t *testing.T) {
	first := []int{0, 1, 2, 3, 4, 5, 6, 7}
	second := append([]int(nil), first...)
	Shuffle(NewMemeDealer(42).rng, first)
	Shuffle(NewMemeDealer(42).rng, second)
	require.Equal(t, first, second)
}

func TestProductionSeedVaries(t *testing.T) {
	first, second := productionSeed(), productionSeed()
	require.NotEqual(t, int64(1), first)
	require.NotEqual(t, first, second)
}

func TestMemeDealerConcurrentCallsAreSafe(t *testing.T) {
	dealer := NewMemeDealer(42)
	const workers = 12
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			agg := &Aggregate{Settings: room.RoomSettings{HandSize: 2}}
			for n := 0; n < 8; n++ {
				agg.GamePlayers = []*game.GamePlayer{{ID: "gp", ParticipationStatus: game.ParticipationActive}}
				agg.Memes = []*content.Meme{{ID: "a", Enabled: true}, {ID: "b", Enabled: true}, {ID: "c", Enabled: true}}
				if err := dealer.DealPreparationHands(agg); err != nil {
					errCh <- err
					return
				}
				dealer.RandomSituation(&Aggregate{Situations: []*content.Situation{{Text: "s", Enabled: true}}})
				options := []*round.VoteOption{{ID: "1"}, {ID: "2"}, {ID: "3"}}
				dealer.ShuffleVoteOptions(options)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}
