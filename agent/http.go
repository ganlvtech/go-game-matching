package agent

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	json "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"

	"github.com/ganlvtech/go-game-matching/matcher"
)

type HttpMatchingServerStats struct {
	ServerStartTime    time.Time
	JoinRequestCount   int64
	StatusRequestCount int64
	LeaveRequestCount  int64
	RemoveRequestCount int64
	BadRequestCount    int64
	ErrorCount         int64
	JoinOKCount        int64
	GetStatusOKCount   int64
}

type HttpMatchingServer struct {
	Matcher       *matcher.Matcher
	mu            sync.Mutex
	Stats         HttpMatchingServerStats
	lastStats     HttpMatchingServerStats
	lastStatsTime time.Time
}

type HttpJsonResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

type MatchingJoinData struct {
	WaitTime int `json:"wait_time"`
}

type MatchingStatusData struct {
	Ids []matcher.PlayerId `json:"ids"`
}

type MatcherStatsData struct {
	PlayerCount            int     `json:"player_count"`
	PlayerInQueueCount     int     `json:"player_in_queue_count"`
	PlayerNotRemovedCount  int     `json:"player_not_removed_count"`
	GroupCount             int     `json:"group_count"`
	GroupStandardDeviation float64 `json:"group_standard_deviation"`
	AverageWaitTime        float64 `json:"average_wait_time"`
	ServerRunningTime      float64 `json:"server_running_time"`
	JoinRequestCount       int     `json:"join_request_count"`
	StatusRequestCount     int     `json:"status_request_count"`
	LeaveRequestCount      int     `json:"leave_request_count"`
	RemoveRequestCount     int     `json:"remove_request_count"`
	BadRequestCount        int     `json:"bad_request_count"`
	ErrorCount             int     `json:"error_count"`
	JoinOKCount            int     `json:"join_ok_count"`
	GetStatusOKCount       int     `json:"get_status_ok_count"`
	JoinRequestQPS         float64 `json:"join_request_qps"`
	StatusRequestQPS       float64 `json:"status_request_qps"`
	LeaveRequestQPS        float64 `json:"leave_request_qps"`
	RemoveRequestQPS       float64 `json:"remove_request_qps"`
	BadRequestQPS          float64 `json:"bad_request_qps"`
	ErrorQPS               float64 `json:"error_qps"`
	JoinOKQPS              float64 `json:"join_ok_qps"`
	GetStatusOKQPS         float64 `json:"get_status_ok_qps"`
}

func NewHttpMatchingServer(maxTime matcher.Time, maxScore matcher.PlayerScore, scoreGroupLen int) *HttpMatchingServer {
	s := &HttpMatchingServer{
		Matcher: matcher.NewMatcher(maxTime, maxScore, scoreGroupLen),
		mu:      sync.Mutex{},
	}
	s.Stats.ServerStartTime = time.Now()
	s.lastStatsTime = s.Stats.ServerStartTime
	return s
}

func (s *HttpMatchingServer) Match(currentTime matcher.Time, count int) {
	s.mu.Lock()
	s.Matcher.Match(currentTime, count)
	s.mu.Unlock()
}

func (s *HttpMatchingServer) Sweep(before matcher.Time) {
	s.mu.Lock()
	s.Matcher.Sweep(before)
	s.mu.Unlock()
}

func (s *HttpMatchingServer) HandleHTTP(ctx *fasthttp.RequestCtx) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	switch string(ctx.Request.URI().Path()) {
	case "/join":
		atomic.AddInt64(&s.Stats.JoinRequestCount, 1)
		s.HandleJoin(ctx)
	case "/status":
		atomic.AddInt64(&s.Stats.StatusRequestCount, 1)
		s.HandleGetStatus(ctx)
	case "/leave":
		atomic.AddInt64(&s.Stats.LeaveRequestCount, 1)
		s.HandleLeave(ctx)
	case "/remove":
		atomic.AddInt64(&s.Stats.RemoveRequestCount, 1)
		s.HandleRemove(ctx)
	case "/stats":
		s.HandleStats(ctx)
	case "/player_ids":
		s.mu.Lock()
		data := s.Matcher.PlayerIds()
		s.mu.Unlock()
		writeJsonResponseOKWithData(ctx, data)
	case "/player_in_queue_ids":
		s.mu.Lock()
		data := s.Matcher.PlayerInQueueIds()
		s.mu.Unlock()
		writeJsonResponseOKWithData(ctx, data)
	case "/group_player_ids":
		s.mu.Lock()
		data := s.Matcher.GroupsPlayerIds()
		s.mu.Unlock()
		writeJsonResponseOKWithData(ctx, data)
	}
}

func (s *HttpMatchingServer) HandleJoin(ctx *fasthttp.RequestCtx) {
	id := matcher.PlayerId(ctx.Request.URI().QueryArgs().Peek("id"))
	score1, err := strconv.Atoi(string(ctx.Request.URI().QueryArgs().Peek("score")))
	if err != nil {
		atomic.AddInt64(&s.Stats.BadRequestCount, 1)
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}
	score := matcher.PlayerScore(score1)
	s.mu.Lock()
	err = s.Matcher.JoinQueue(id, matcher.Time(time.Now().Unix()), score)
	s.mu.Unlock()
	if err != nil {
		log.Println(ctx.RemoteIP(), ctx.RequestURI(), err)
		atomic.AddInt64(&s.Stats.ErrorCount, 1)
		writeJsonResponseError(ctx, 1, err)
		return
	}
	waitTime := s.Matcher.GetWaitTimeByScore(score)
	atomic.AddInt64(&s.Stats.JoinOKCount, 1)
	writeJsonResponseOKWithData(ctx, MatchingJoinData{WaitTime: waitTime,})
}

func (s *HttpMatchingServer) HandleGetStatus(ctx *fasthttp.RequestCtx) {
	id := matcher.PlayerId(ctx.Request.URI().QueryArgs().Peek("id"))
	s.mu.Lock()
	ids, err := s.Matcher.GetMatchedPlayers(id)
	s.mu.Unlock()
	if err != nil {
		log.Println(ctx.RemoteIP(), ctx.RequestURI(), err)
		atomic.AddInt64(&s.Stats.ErrorCount, 1)
		writeJsonResponseError(ctx, 2, err)
		return
	}
	atomic.AddInt64(&s.Stats.GetStatusOKCount, 1)
	writeJsonResponseOKWithData(ctx, MatchingStatusData{Ids: ids})
}

func (s *HttpMatchingServer) HandleLeave(ctx *fasthttp.RequestCtx) {
	id := matcher.PlayerId(ctx.Request.URI().QueryArgs().Peek("id"))
	s.mu.Lock()
	err := s.Matcher.LeaveQueue(id)
	s.mu.Unlock()
	if err != nil {
		log.Println(ctx.RemoteIP(), ctx.RequestURI(), err)
		atomic.AddInt64(&s.Stats.ErrorCount, 1)
		writeJsonResponseError(ctx, 3, err)
		return
	}
	writeJsonResponseOK(ctx)
}

func (s *HttpMatchingServer) HandleRemove(ctx *fasthttp.RequestCtx) {
	id := matcher.PlayerId(ctx.Request.URI().QueryArgs().Peek("id"))
	s.mu.Lock()
	s.Matcher.Remove(id)
	s.mu.Unlock()
	writeJsonResponseOK(ctx)
}

func (s *HttpMatchingServer) HandleStats(ctx *fasthttp.RequestCtx) {
	s.mu.Lock()
	data := &MatcherStatsData{
		PlayerCount:            s.Matcher.PlayerCount(),
		PlayerInQueueCount:     s.Matcher.PlayerInQueueCount(),
		PlayerNotRemovedCount:  s.Matcher.PlayerNotRemovedCount(),
		GroupCount:             s.Matcher.GroupCount(),
		GroupStandardDeviation: s.Matcher.GroupStandardDeviation(),
		AverageWaitTime:        s.Matcher.AverageWaitTime(),
	}
	s.mu.Unlock()
	now := time.Now()
	data.ServerRunningTime = now.Sub(s.Stats.ServerStartTime).Seconds()
	data.JoinRequestCount = int(s.Stats.JoinRequestCount)
	data.StatusRequestCount = int(s.Stats.StatusRequestCount)
	data.LeaveRequestCount = int(s.Stats.LeaveRequestCount)
	data.RemoveRequestCount = int(s.Stats.RemoveRequestCount)
	data.BadRequestCount = int(s.Stats.BadRequestCount)
	data.ErrorCount = int(s.Stats.ErrorCount)
	data.JoinOKCount = int(s.Stats.JoinOKCount)
	data.GetStatusOKCount = int(s.Stats.GetStatusOKCount)
	data.JoinRequestQPS = float64(s.Stats.JoinRequestCount-s.lastStats.JoinRequestCount) / now.Sub(s.lastStatsTime).Seconds()
	data.StatusRequestQPS = float64(s.Stats.StatusRequestCount-s.lastStats.StatusRequestCount) / now.Sub(s.lastStatsTime).Seconds()
	data.LeaveRequestQPS = float64(s.Stats.LeaveRequestCount-s.lastStats.LeaveRequestCount) / now.Sub(s.lastStatsTime).Seconds()
	data.RemoveRequestQPS = float64(s.Stats.RemoveRequestCount-s.lastStats.RemoveRequestCount) / now.Sub(s.lastStatsTime).Seconds()
	data.BadRequestQPS = float64(s.Stats.BadRequestCount-s.lastStats.BadRequestCount) / now.Sub(s.lastStatsTime).Seconds()
	data.ErrorQPS = float64(s.Stats.ErrorCount-s.lastStats.ErrorCount) / now.Sub(s.lastStatsTime).Seconds()
	data.JoinOKQPS = float64(s.Stats.JoinOKCount-s.lastStats.JoinOKCount) / now.Sub(s.lastStatsTime).Seconds()
	data.GetStatusOKQPS = float64(s.Stats.GetStatusOKCount-s.lastStats.GetStatusOKCount) / now.Sub(s.lastStatsTime).Seconds()
	writeJsonResponseOKWithData(ctx, data)
	s.lastStats.JoinRequestCount = s.Stats.JoinRequestCount
	s.lastStats.StatusRequestCount = s.Stats.StatusRequestCount
	s.lastStats.LeaveRequestCount = s.Stats.LeaveRequestCount
	s.lastStats.RemoveRequestCount = s.Stats.RemoveRequestCount
	s.lastStats.BadRequestCount = s.Stats.BadRequestCount
	s.lastStats.ErrorCount = s.Stats.ErrorCount
	s.lastStats.JoinOKCount = s.Stats.JoinOKCount
	s.lastStats.GetStatusOKCount = s.Stats.GetStatusOKCount
	s.lastStatsTime = now
}

func writeJsonResponse(ctx *fasthttp.RequestCtx, v interface{}) {
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(v)
}

func writeJsonResponseError(ctx *fasthttp.RequestCtx, code int, err error) {
	writeJsonResponse(ctx, &HttpJsonResponse{
		Code: code,
		Msg:  err.Error(),
	})
}

func writeJsonResponseOK(ctx *fasthttp.RequestCtx) {
	writeJsonResponseOKWithData(ctx, nil)
}

func writeJsonResponseOKWithData(ctx *fasthttp.RequestCtx, data interface{}) {
	ctx.SetContentType("application/json")
	_ = json.NewEncoder(ctx).Encode(&HttpJsonResponse{
		Code: 0,
		Msg:  "OK",
		Data: data,
	})
}
