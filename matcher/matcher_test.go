package matcher_test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ganlvtech/go-game-matching/matcher"
)

func benchmarkMatcher_Match(b *testing.B, concurrent int) {
	const maxScore = 300
	var m = matcher.NewMatcher(120, maxScore, 10)
	var mu sync.Mutex
	isRun := true

	for i := 0; i < concurrent; i++ {
		go func(base int) {
			for id := base; isRun; id++ {
				score := rand.Uint32() % maxScore
				mu.Lock()
				err := m.JoinQueue(matcher.PlayerId(strconv.Itoa(id)), matcher.Time(time.Now().Unix()), matcher.PlayerScore(score))
				mu.Unlock()
				if err != nil {
					panic(err)
				}
			}
		}(i * 1000000)
	}

	for {
		mu.Lock()
		m.Match(matcher.Time(time.Now().Unix()), 25)
		mu.Unlock()
		if m.PlayerCount()-m.PlayerInQueueCount() > b.N {
			isRun = false
			break
		}
		time.Sleep(time.Microsecond * 1000)
	}
}

// -test.benchtime 10s
func BenchmarkMatcher_Match(b *testing.B) {
	benchmarkMatcher_Match(b, 1)
}

// -test.benchtime 10s
func BenchmarkMatcher_Match_1000(b *testing.B) {
	benchmarkMatcher_Match(b, 1000)
}
