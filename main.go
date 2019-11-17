package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/ganlvtech/go-game-matching/agent"
	"github.com/ganlvtech/go-game-matching/matcher"
)

func main() {
	matchingServer := agent.NewHttpMatchingServer(180, 300, 10)
	server := fasthttp.Server{
		Handler: matchingServer.HandleHTTP,
		Name:    "go-game-matching",
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
			matchingServer.Match(matcher.Time(time.Now().Unix()), 25)
			time.Sleep(time.Second)
		}
	}()

	go func() {
		log.Println("Sweeping service started.")
		for isRun {
			log.Println("Before sweeping.", matchingServer.Matcher.PlayerCount())
			matchingServer.Sweep(matcher.Time(time.Now().Unix() - 10))
			runtime.GC()
			debug.FreeOSMemory()
			log.Println("After sweeping.", matchingServer.Matcher.PlayerCount())
			time.Sleep(10 * time.Second)
		}
	}()

	log.Println("HTTP server started.")
	addr := ":8000"
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}
	if err := server.ListenAndServe(addr); err != nil {
		log.Fatal(err)
	}
	log.Println("Server exit.")
}
