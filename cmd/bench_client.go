package main

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"

	json "github.com/json-iterator/go"

	"github.com/ganlvtech/go-game-matching/agent"
)

func main() {
	const concurrent = 50
	isRun := true
	for i := 0; i < concurrent; i++ {
		go func(base int) {
			tr := &http.Transport{}
			client := &http.Client{Transport: tr}
			for id := base; isRun; id++ {
				score := int(150 + 50*rand.NormFloat64())
				if score < 0 {
					score = 0
				} else if score >= 300 {
					score = 299
				}
				resp, err := client.Get("http://127.0.0.1:8000/join?id=" + strconv.Itoa(id) + "&score=" + strconv.Itoa(score))
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
				time.Sleep(time.Duration(rand.Uint32()%2000) * time.Millisecond)
			}
		}(i * 1000000)
	}
	for {
		resp, err := http.Get("http://127.0.0.1:8000/stats")
		if err != nil {
			panic(err)
		}
		// bytes, err := ioutil.ReadAll(resp.Body)
		// fmt.Println(string(bytes))
		r := &agent.HttpJsonResponse{}
		err = json.NewDecoder(resp.Body).Decode(r)
		if err != nil {
			panic(err)
		}
		i := r.Data.(map[string]interface{})
		playerCount := i["player_count"].(float64)
		playerInQueueCount := i["player_in_queue_count"].(float64)
		if playerCount-playerInQueueCount > 100000 {
			isRun = false
			break
		}
		time.Sleep(time.Second)
	}
}
