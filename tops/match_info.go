package tops

import (
	"errors"
	"sync"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
)

type MatchInformer struct {
	mu sync.RWMutex
}

var MatchInfo *MatchInformer

func init() {
	MatchInfo = &MatchInformer{}
}

func (i *MatchInformer) matchFinished(match *got.Match) bool {
	_, err := match.GetWinner()
	// Ignoring ErrEqualScore, is not allowed in badminton
	return !errors.Is(err, got.ErrNoScore)
}

func (i *MatchInformer) MatchFinished(match *got.Match) bool {
	defer i.mu.RUnlock()
	i.mu.RLock()

	return i.matchFinished(match)
}

func (i *MatchInformer) MatchesFinished(matches []*got.Match) bool {
	defer i.mu.RUnlock()
	i.mu.RLock()

	for _, match := range matches {
		if !i.matchFinished(match) {
			return false
		}
	}
	return true
}

func (i *MatchInformer) MatchRunning(match *got.Match) bool {
	defer i.mu.RUnlock()
	i.mu.RLock()

	return !match.StartTime.IsZero() && !i.matchFinished(match)
}

func (i *MatchInformer) MatchStarted(match *got.Match) bool {
	defer i.mu.RUnlock()
	i.mu.RLock()

	return !match.StartTime.IsZero()
}

func (i *MatchInformer) HasCourt(match *got.Match) bool {
	defer i.mu.RUnlock()
	i.mu.RLock()

	return match.Location != nil
}

func (i *MatchInformer) PlayersInMatch(match *got.Match) []*Player {
	defer i.mu.RUnlock()
	i.mu.RLock()

	players := make([]*Player, 0, 4)
	for s := range match.Slots {
		if s.Player == nil {
			continue
		}
		team := s.Player.(*TournamentPlayer).Team
		players = append(players, team.Players()...)
	}

	return players
}
