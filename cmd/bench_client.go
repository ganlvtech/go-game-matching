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
	const concurrent = 100
	isRun := true
	for i := 0; i < concurrent; i++ {
		go func(base int) {
			time.Sleep(1000 * time.Millisecond)
			tr := &http.Transport{}
			client := &http.Client{Transport: tr}
			for id := base; isRun; id++ {
				score := int(rand.Uint32()%300)
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
