package agent_test

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/ganlvtech/go-game-matching/agent"
	"github.com/ganlvtech/go-game-matching/matcher"
)

func benchmarkHttpMatchingServer_HandleJoin(b *testing.B, concurrent int) {
	const maxScore = 300
	isRun := true
	port := int(9000 + rand.Uint32()%1000)

	matchingServer := agent.NewHttpMatchingServer(180, maxScore, 10)

	go func() {
		server := fasthttp.Server{
			Handler: matchingServer.HandleHTTP,
			Name:    "go-game-matching",
		}

		if err := server.ListenAndServe("127.0.0.1:" + strconv.Itoa(port)); err != nil {
			panic(err)
		}
	}()

	for i := 0; i < concurrent; i++ {
		go func(base int) {
			time.Sleep(1000 * time.Millisecond)
			tr := &http.Transport{}
			client := &http.Client{Transport: tr}
			for id := base; isRun; id++ {
				score := int(rand.Uint32()%250 + 25)
				resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/join?id=" + strconv.Itoa(id) + "&score=" + strconv.Itoa(score))
				if err != nil {
					panic(err)
				}
				decoder := json.NewDecoder(resp.Body)
				r := &agent.HttpJsonResponse{}
				err = decoder.Decode(r)
				if err != nil {
					panic(err)
				}
				_ = resp.Body.Close()
				if r.Code != 0 {
					panic(r.Msg)
				}
			}
		}(i * 1000000)
	}

	for {
		matchingServer.Match(matcher.Time(time.Now().Unix()), 25)
		if matchingServer.Matcher.PlayerCount()-matchingServer.Matcher.PlayerInQueueCount() > b.N {
			isRun = false
			break
		}
	}
}

func BenchmarkHttpMatchingServer_HandleJoin(b *testing.B) {
	benchmarkHttpMatchingServer_HandleJoin(b, 1)
}

func BenchmarkHttpMatchingServer_HandleJoin_100(b *testing.B) {
	benchmarkHttpMatchingServer_HandleJoin(b, 100)
}
