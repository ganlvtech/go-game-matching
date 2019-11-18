package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/ganlvtech/go-game-matching/agent"
	"github.com/ganlvtech/go-game-matching/matcher"
)

var maxTime int
var maxScore int
var scoreGroupLen int
var matchCount int

func init() {
	flag.IntVar(&maxTime, "max_time", 180, "最长匹配时间，超过时间会被移出匹配队列，不超过二倍时间会保留玩家匹配信息，超过二倍时间会自动清除该角色的所有信息")
	flag.IntVar(&maxScore, "max_score", 300, "最大分数")
	flag.IntVar(&scoreGroupLen, "score_group_len", 10, "每一分段长度，越短匹配越精确，稍长性能会好，但是过长性能会很差")
	flag.IntVar(&matchCount, "match_count", 25, "每组匹配人数")
}

func main() {
	flag.Parse()

	matchingServer := agent.NewHttpMatchingServer(matcher.Time(maxTime), matcher.PlayerScore(maxScore), scoreGroupLen)
	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			if string(ctx.Request.URI().Path()) == "/" {
				b, err := ioutil.ReadFile("web/index.html")
				if err != nil {
					ctx.SetStatusCode(404)
					return
				}
				ctx.SetStatusCode(200)
				ctx.SetContentType("text/html; charset=utf-8")
				_, _ = ctx.Write(b)
				return
			}
			matchingServer.HandleHTTP(ctx)
		},
		Name: "go-game-matching",
	}
	isRun := true

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Println("Server is shutting down.")
		isRun = false
		if err := server.Shutdown(); err != nil {
			log.Fatal(err)
		}
		log.Println("Server shutdown finished.")
	}()

	go func() {
		log.Println("Matching service started.")
		for isRun {
			matchingServer.Match(matcher.Time(time.Now().Unix()), matchCount)
			time.Sleep(time.Second)
		}
	}()

	go func() {
		log.Println("Sweeping service started.")
		for isRun {
			matchingServer.Sweep(matcher.Time(int(time.Now().Unix()) - maxTime*2))
			time.Sleep(time.Duration(maxTime) * time.Second)
		}
	}()

	addr := ":8000"
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}
	log.Println("HTTP server listening " + addr)
	if err := server.ListenAndServe(addr); err != nil {
		log.Fatal(err)
	}
	log.Println("Server main exit.")
}
