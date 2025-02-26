package tops

import (
	"slices"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
)

type PlayerTracker struct {
	// player id -> currently played match
	inMatch map[string]*MatchData
	// player id -> last ended match
	lastMatches     map[string]*MatchData
	restTime        time.Duration
	tournamentStore *TournamentStore
}

func newPlayerTracker(tournamentStore *TournamentStore) *PlayerTracker {
	tournamentEventStore, _ := store.FindRecordStore[TournamentEvent]()

	runningTournaments := tournamentStore.listStarted()
	matches := make([]*got.Match, 0)
	for _, t := range runningTournaments {
		matches = append(matches, t.MatchList().Matches...)
	}
	slices.SortFunc(matches, compareMatchEndTimes)

	tournamentEvent := tournamentEventStore.RecordList[0]
	restMinutes := tournamentEvent.PlayerRestTime()

	playerTracker := &PlayerTracker{
		restTime:        time.Duration(restMinutes) * time.Minute,
		tournamentStore: tournamentStore,
	}
	playerTracker.inMatch = playerTracker.collectCurrentMatches(matches)
	playerTracker.lastMatches = playerTracker.collectLastMatches(matches)

	return playerTracker
}

func (t *PlayerTracker) collectCurrentMatches(matches []*got.Match) map[string]*MatchData {
	inMatch := make(map[string]*MatchData)
	for _, m := range matches {
		if !matchRunning(m) {
			continue
		}

		matchData := t.tournamentStore.matchData[m.Id()]
		players := playersInMatch(m)

		for _, p := range players {
			inMatch[p.Id] = matchData
		}
	}
	return inMatch
}

// Goes through the sortedMatches (sorted by end time) and
// maps each player ID to their last ended match
func (t *PlayerTracker) collectLastMatches(sortedMatches []*got.Match) map[string]*MatchData {
	lastMatches := make(map[string]*MatchData)
	for _, m := range sortedMatches {
		if m.EndTime.IsZero() {
			break
		}

		matchData := t.tournamentStore.matchData[m.Id()]
		players := playersInMatch(m)

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

func (t *PlayerTracker) isPlaying(player *Player) (bool, *MatchData) {
	match, ok := t.inMatch[player.Id]
	if !ok {
		return false, nil
	}
	return true, match
}

func (t *PlayerTracker) isResting(player *Player) (bool, time.Time) {
	lastMatch, ok := t.lastMatches[player.Id]
	if !ok {
		return false, time.Time{}
	}

	restUntil := lastMatch.EndTime().Add(t.restTime)
	isResting := time.Now().Before(restUntil.Time())

	return isResting, restUntil.Time()
}

func compareMatchEndTimes(a, b *got.Match) int {
	// Sort by most recently ended
	return b.EndTime.Compare(a.EndTime)
}
