package matcher

import (
	"math"

	"github.com/wangjia184/sortedset"
	"golang.org/x/tools/container/intsets"
)

const (
	TimeGroupCount = 60
)

type PlayerId string
type Time int
type PlayerScore uint
type OnGroupMatchedEventCallback func(group *Group)
type ScoreRadiusFunc func(deltaT Time) PlayerScore

type Group struct {
	Players      []*Player
	removed      []bool
	removedCount int
}

func NewGroup(count int) *Group {
	return &Group{
		Players:      make([]*Player, count),
		removed:      make([]bool, count),
		removedCount: 0,
	}
}

func (g *Group) isEmpty() bool {
	return g.removedCount >= len(g.Players)
}

func (g *Group) softRemove(p *Player) {
	for i, v := range g.Players {
		if v == p && !g.removed[i] {
			g.removed[i] = true
			g.removedCount++
		}
	}
}

func (g *Group) PlayerIds() []PlayerId {
	s := make([]PlayerId, len(g.Players))
	i := 0
	for _, p := range g.Players {
		s[i] = p.Id
		i++
	}
	return s
}

// 管理统计函数 ==========

func (g *Group) PlayerNotRemovedCount() int {
	sum := 0
	for i := range g.Players {
		if !g.removed[i] {
			sum++
		}
	}
	return sum
}

func (g *Group) PlayerNotRemovedIds() []PlayerId {
	s := make([]PlayerId, 0, len(g.Players))
	for i, p := range g.Players {
		if !g.removed[i] {
			s = append(s, p.Id)
		}
	}
	return s
}

func (g *Group) StandardDeviation() float64 {
	sum := float64(0)
	count := 0
	for _, p := range g.Players {
		sum += float64(p.Score)
		count++
	}
	if count <= 0 {
		return 0
	}
	mean := sum / float64(count)

	sum = 0
	for _, p := range g.Players {
		sum += math.Pow(float64(p.Score)-mean, 2)
	}
	sd := math.Sqrt(sum / float64(count))
	return sd
}

func (g *Group) AverageWaitTime(currentTime Time) float64 {
	sum := 0
	count := 0
	for _, p := range g.Players {
		sum += int(currentTime - p.JoinTime)
		count++
	}
	if count <= 0 {
		return 0
	}
	mean := float64(sum) / float64(count)
	return mean
}

// 管理统计函数结束 ==========

type Player struct {
	Id       PlayerId
	JoinTime Time
	gridX    int
	Score    PlayerScore
	Group    *Group
}

type Matcher struct {
	players                     map[PlayerId]*Player // 全部玩家
	playerQueue                 *sortedset.SortedSet // 未匹配的玩家队列
	timeScoreGrid               *GeoHash             // 为匹配的玩家二维 Hash 表
	groups                      []*Group             // 已匹配成功的队列
	waitTime                    *WaitTime            // 分组等待时间
	ScoreRadiusFunc             ScoreRadiusFunc
	OnGroupMatchedEventCallback OnGroupMatchedEventCallback
}

func NewMatcher(maxTime Time, maxScore PlayerScore, scoreGroupLen int) *Matcher {
	timeGroupLen := int(maxTime) / TimeGroupCount
	if timeGroupLen < 3 {
		timeGroupLen = 3
	} else if timeGroupLen > 10 {
		timeGroupLen = 10
	}
	timeGroupCount := int(maxTime) / timeGroupLen
	if scoreGroupLen < 1 {
		timeGroupLen = 1
	}
	scoreGroupCount := int(maxScore) / scoreGroupLen
	if scoreGroupCount > 1000 { // 请避免分组过多，既消耗大量内存，遍历性能又低
		return nil
	}
	return &Matcher{
		players:       make(map[PlayerId]*Player),
		playerQueue:   sortedset.New(),
		timeScoreGrid: NewGeoHash(timeGroupCount, scoreGroupCount, timeGroupLen, scoreGroupLen),
		groups:        make([]*Group, 0, 64),
		waitTime:      NewWaitTime(scoreGroupCount, float64(maxTime)),
		ScoreRadiusFunc: func(deltaT Time) PlayerScore {
			if deltaT > maxTime {
				return maxScore
			} else {
				return maxScore/8 + maxScore*PlayerScore(deltaT)/PlayerScore(maxTime*7/8)
			}
		},
	}
}

func (m *Matcher) Exists(id PlayerId) bool {
	_, ok := m.players[id]
	return ok
}

func (m *Matcher) IsMatched(id PlayerId) (bool, error) {
	p, ok := m.players[id]
	if !ok {
		return false, PlayerNotExistsError(id)
	}
	matched := p.Group != nil
	return matched, nil
}

func (m *Matcher) timeToGridX(joinTime Time) int {
	return int(joinTime) % m.timeScoreGrid.XLen()
}

// 加入队列，用于未开始匹配的玩家
func (m *Matcher) JoinQueue(id PlayerId, joinTime Time, score PlayerScore) error {
	if m.Exists(id) {
		return PlayerAlreadyExistsError(id)
	}
	p := &Player{
		Id:       id,
		JoinTime: joinTime,
		gridX:    m.timeToGridX(joinTime),
		Score:    score,
		Group:    nil,
	}
	m.players[id] = p
	m.playerQueue.AddOrUpdate(string(p.Id), sortedset.SCORE(joinTime), p)
	m.timeScoreGrid.Add(p.gridX, int(score), p)
	return nil
}

// 离开队列，用于仍在匹配中的玩家
func (m *Matcher) LeaveQueue(id PlayerId) error {
	p, ok := m.players[id]
	if !ok {
		return PlayerNotExistsError(id)
	}
	// 如果匹配完成中，禁止离开队列
	if p.Group != nil {
		return PlayerAlreadyMatchedError(id)
	}
	m.playerQueue.Remove(string(id))
	m.timeScoreGrid.Del(p.gridX, int(p.Score), p)
	delete(m.players, id)
	return nil
}

// 删除用户，用于已匹配到的玩家（如果玩家仍在匹配中，则会强制移出队列）
func (m *Matcher) Remove(id PlayerId) {
	p, ok := m.players[id]
	if ok {
		m.playerQueue.Remove(string(id))
		m.timeScoreGrid.Del(p.gridX, int(p.Score), p)
		delete(m.players, id)
		if p.Group != nil {
			p.Group.softRemove(p)
			if p.Group.isEmpty() {
				m.removeGroup(p.Group)
			}
		}
	}
}

func (m *Matcher) removeGroup(g *Group) {
	for i, v := range m.groups {
		if v == g {
			if i != len(m.groups)-1 {
				m.groups[i] = m.groups[len(m.groups)-1]
			}
			m.groups = m.groups[:len(m.groups)-1]
			break
		}
	}
}

func (m *Matcher) IterPlayerCandidates(p *Player, startTime Time, currentTime Time, scoreRadius PlayerScore, iterFunc func(v interface{}) bool) {
	h := m.timeScoreGrid
	startI := h.GetXGroupIndex(m.timeToGridX(startTime))
	endI := h.GetXGroupIndex(m.timeToGridX(currentTime))
	middleJ := h.GetYGroupIndex(int(p.Score))
	jRadius := int(scoreRadius) / h.YGroupLen
Outer:
	for i2 := startI; ; i2++ {
		if i2 >= h.XCount {
			i2 -= h.XCount
		}
		// 分数从中间向两侧取
		for j := 0; j <= jRadius; j++ {
			if j == 0 {
				// 由于 j == 0 所以 j2 = middleJ + j = middleJ
				for _, v := range h.Data[i2][middleJ] {
					if iterFunc(v) {
						break Outer
					}
				}
			} else {
				j2 := middleJ - j
				if j2 >= 0 {
					for _, v := range h.Data[i2][j2] {
						if iterFunc(v) {
							break Outer
						}
					}
				}
				j2 = middleJ + j
				if j2 < h.YCount {
					for _, v := range h.Data[i2][j2] {
						if iterFunc(v) {
							break Outer
						}
					}
				}
			}
		}
		if i2 == endI {
			break
		}
	}
}

func (m *Matcher) MatchForPlayer(id PlayerId, currentTime Time, count int) error {
	p, ok := m.players[id]
	if !ok {
		return PlayerNotExistsError(id)
	}
	if p.Group != nil {
		return PlayerAlreadyMatchedError(id)
	}
	g := NewGroup(count)
	i := 0
	scoreRadius := m.ScoreRadiusFunc(currentTime - p.JoinTime)
	startTime := Time(m.playerQueue.GetByRank(1, false).Score())
	m.IterPlayerCandidates(p, startTime, currentTime, scoreRadius, func(v interface{}) bool {
		candidate := v.(*Player)
		if candidate.Group == nil {
			g.Players[i] = candidate
			i++
		}
		if i >= count {
			m.groups = append(m.groups, g)
			for _, matchedPlayer := range g.Players {
				m.playerQueue.Remove(string(matchedPlayer.Id))
				m.timeScoreGrid.Del(matchedPlayer.gridX, int(matchedPlayer.Score), matchedPlayer)
				matchedPlayer.Group = g
				m.waitTime.AddItem(m.timeScoreGrid.GetYGroupIndex(int(matchedPlayer.Score)), float64(currentTime-matchedPlayer.JoinTime))
			}
			if m.OnGroupMatchedEventCallback != nil {
				m.OnGroupMatchedEventCallback(g)
			}
			return true
		}
		return false
	})
	return nil
}

func (m *Matcher) getMinTime(currentTime Time) Time {
	return currentTime - Time(m.timeScoreGrid.XLen())
}

func (m *Matcher) AutoRemove(currentTime Time) {
	for _, v := range m.playerQueue.GetByScoreRange(sortedset.SCORE(intsets.MinInt), sortedset.SCORE(m.getMinTime(currentTime)), &sortedset.GetByScoreRangeOptions{ExcludeEnd: true}) {
		m.Remove(PlayerId(v.Key()))
	}
}

func (m *Matcher) Match(currentTime Time, count int) {
	m.AutoRemove(currentTime)
	m.waitTime.AddTimeAuto(float64(currentTime))

	for _, v := range m.playerQueue.GetByScoreRange(sortedset.SCORE(intsets.MinInt), sortedset.SCORE(currentTime), nil) {
		_ = m.MatchForPlayer(PlayerId(v.Key()), currentTime, count)
	}

	m.waitTime.Merge()
}

func (m *Matcher) GetMatchedPlayers(id PlayerId) ([]PlayerId, error) {
	p, ok := m.players[id]
	if !ok {
		return nil, PlayerNotExistsError(id)
	}
	if p.Group == nil {
		return nil, PlayerNotMatchedError(id)
	}
	return p.Group.PlayerIds(), nil
}

func (m *Matcher) GetPlayerApproxWaitTime(id PlayerId) (int, error) {
	p, ok := m.players[id]
	if !ok {
		return 0, PlayerNotExistsError(id)
	}
	if p.Group != nil {
		return 0, PlayerAlreadyMatchedError(id)
	}
	return m.GetWaitTimeByScore(p.Score), nil
}

func (m *Matcher) GetWaitTimeByScore(score PlayerScore) int {
	return int(m.waitTime.Groups[m.timeScoreGrid.GetYGroupIndex(int(score))])
}

func (m *Matcher) Sweep(before Time) {
	for id := range m.players {
		if m.players[id].JoinTime < before {
			m.Remove(id)
		}
	}
}

// 仅用于管理的函数 ==========

func (m *Matcher) Players() map[PlayerId]*Player {
	return m.players
}

func (m *Matcher) PlayerCount() int {
	return len(m.players)
}

func (m *Matcher) PlayerIds() []PlayerId {
	s := make([]PlayerId, len(m.players))
	i := 0
	for id := range m.players {
		s[i] = id
		i++
	}
	return s
}

func (m *Matcher) PlayerInQueueCount() int {
	return m.playerQueue.GetCount()
}

func (m *Matcher) PlayerInQueueIds() []PlayerId {
	s := make([]PlayerId, m.playerQueue.GetCount())
	i := 0
	for _, sortedSetNode := range m.playerQueue.GetByRankRange(1, -1, false) {
		s[i] = PlayerId(sortedSetNode.Key())
		i++
	}
	return s
}

func (m *Matcher) PlayerNotRemovedCount() int {
	sum := 0
	for _, g := range m.groups {
		sum += g.PlayerNotRemovedCount()
	}
	return sum
}

func (m *Matcher) Groups() []*Group {
	return m.groups
}

func (m *Matcher) GroupCount() int {
	return len(m.groups)
}

func (m *Matcher) GroupStandardDeviation() float64 {
	sum := float64(0)
	count := 0
	for _, g := range m.groups {
		sum += g.StandardDeviation()
		count++
	}
	if count <= 0 {
		return 0
	}
	return sum / float64(count)
}

func (m *Matcher) GroupsPlayerIds() [][]PlayerId {
	s := make([][]PlayerId, len(m.groups))
	for i, g := range m.groups {
		s[i] = g.PlayerIds()
	}
	return s
}

func (m *Matcher) AverageWaitTime() float64 {
	sum := float64(0)
	for _, t := range m.waitTime.Groups {
		sum += t
	}
	return sum / float64(m.waitTime.GroupCount())
}

// 仅用于管理的函数结束 ==========
