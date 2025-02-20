package tops

import (
	"slices"
	"sync"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
)

type PlayerOccupationTracker struct {
	// player id -> currently played match
	inMatch map[string]*MatchData
	// player id -> last ended match
	lastMatches map[string]*MatchData
	restTime    time.Duration
	mu          sync.RWMutex
}

var PlayerTracker *PlayerOccupationTracker

func InitPlayerTracker() error {
	tournamentEventStore, err := store.FindRecordStore[TournamentEvent]()
	if err != nil {
		return err
	}

	runningTournaments := Tournaments.ListRunning()
	matches := make([]*got.Match, 0)
	for _, t := range runningTournaments {
		matches = append(matches, t.MatchList().Matches...)
	}
	slices.SortFunc(matches, compareMatchEndTimes)

	tournamentEvent := tournamentEventStore.RecordList[0]
	restMinutes := tournamentEvent.PlayerRestTime()

	PlayerTracker = &PlayerOccupationTracker{
		inMatch:     collectCurrentMatches(matches),
		lastMatches: collectLastMatches(matches),
		restTime:    time.Duration(restMinutes) * time.Minute,
	}

	return nil
}

func collectCurrentMatches(matches []*got.Match) map[string]*MatchData {
	inMatch := make(map[string]*MatchData)
	for _, m := range matches {
		if !MatchInfo.MatchRunning(m) {
			continue
		}

		matchData := Tournaments.FindMatchData(m)
		players := MatchInfo.PlayersInMatch(m)

		for _, p := range players {
			inMatch[p.Id] = matchData
		}
	}
	return inMatch
}

// Goes through the sortedMatches (sorted by end time) and
// maps each player ID to their last ended match
func collectLastMatches(sortedMatches []*got.Match) map[string]*MatchData {
	lastMatches := make(map[string]*MatchData)
	for _, m := range sortedMatches {
		if m.EndTime.IsZero() {
			break
		}

		matchData := Tournaments.FindMatchData(m)
		players := MatchInfo.PlayersInMatch(m)

		for _, p := range players {
			_, ok := lastMatches[p.Id]
			if !ok {
				lastMatches[p.Id] = matchData
			}
		}
	}
	return lastMatches
}

// TODO: track ending matches

func (t *PlayerOccupationTracker) IsPlaying(player *Player) (bool, *MatchData) {
	defer t.mu.RUnlock()
	t.mu.RLock()

	match, ok := t.inMatch[player.Id]
	if !ok {
		return false, nil
	}
	return true, match
}

func (t *PlayerOccupationTracker) IsResting(player *Player) bool {
	defer t.mu.RUnlock()
	t.mu.RLock()

	lastMatch, ok := t.lastMatches[player.Id]
	if !ok {
		return false
	}

	restUntil := lastMatch.EndTime().Add(t.restTime)
	isResting := time.Now().Before(restUntil.Time())

	return isResting
}

func compareMatchEndTimes(a, b *got.Match) int {
	// Sort by most recently ended
	return b.EndTime.Compare(a.EndTime)
}
