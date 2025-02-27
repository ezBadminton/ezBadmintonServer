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

func newPlayerTracker(
	tournamentStore *TournamentStore,
	courtStore *CourtStore,
	matchManager *MatchManager,
) *PlayerTracker {
	tournamentEventStore, _ := store.FindRecordStore[TournamentEvent]()

	runningTournaments := tournamentStore.listStarted()
	matches := make([]*got.Match, 0)
	for _, t := range runningTournaments {
		matches = append(matches, t.MatchList().Matches...)
	}
	slices.SortFunc(matches, compareMatchEndTimes)

	tournamentEvent := tournamentEventStore.RecordList[0]
	restMinutes := tournamentEvent.PlayerRestTime()

	t := &PlayerTracker{
		restTime:        time.Duration(restMinutes) * time.Minute,
		tournamentStore: tournamentStore,
		inMatch:         collectCurrentMatches(matches, tournamentStore),
		lastMatches:     collectLastMatches(matches, tournamentStore),
	}

	courtStore.onAfterCourtAssign.BindFunc(t.handleCourtAssignment)
	courtStore.onAfterCourtUnassign.BindFunc(t.handleCourtUnassignment)

	// The handler priority is set to 1 here so they are executed before the
	// scheduler handles the same event (with default prio 0). The scheduler
	// needs the player tracker to be updated before it updates the schedule.
	matchManager.onAfterScoreSet.Bind(priorityHandler(t.handleScoreSet, 1))

	tournamentStore.onAfterStop.Bind(priorityHandler(t.handleTournamentStop, 1))

	return t
}

func collectCurrentMatches(matches []*got.Match, tournamentStore *TournamentStore) map[string]*MatchData {
	inMatch := make(map[string]*MatchData)
	for _, m := range matches {
		if !matchRunning(m) {
			continue
		}

		matchData := tournamentStore.matchData[m.Id()]
		players := playersInMatch(m)

		for _, p := range players {
			inMatch[p.Id] = matchData
		}
	}
	return inMatch
}

// Goes through the sortedMatches (sorted by end time) and
// maps each player ID to their last ended match
func collectLastMatches(sortedMatches []*got.Match, tournamentStore *TournamentStore) map[string]*MatchData {
	lastMatches := make(map[string]*MatchData)
	for _, m := range sortedMatches {
		if m.EndTime.IsZero() {
			break
		}

		matchData := tournamentStore.matchData[m.Id()]
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

func (t *PlayerTracker) handleCourtAssignment(e *CourtEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	players := playersInMatch(e.Match)
	for _, p := range players {
		t.inMatch[p.Id] = e.MatchData
	}
	return nil
}

func (t *PlayerTracker) handleCourtUnassignment(e *CourtEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	players := playersInMatch(e.Match)
	for _, p := range players {
		delete(t.inMatch, p.Id)
	}
	return nil
}

func (t *PlayerTracker) handleScoreSet(e *ScoreEvent) error {
	matchEnding := e.MatchData.EndTime().IsZero()

	if err := e.Next(); err != nil {
		return err
	}

	if !matchEnding {
		return nil
	}
	players := playersInMatch(e.Match)
	for _, p := range players {
		delete(t.inMatch, p.Id)
		t.lastMatches[p.Id] = e.MatchData
	}
	return nil
}

func (t *PlayerTracker) handleTournamentStop(e *PlanEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	matches := e.Tournament.MatchList().Matches
	for _, match := range matches {
		if match.Location == nil || !match.EndTime.IsZero() {
			continue
		}
		players := playersInMatch(match)
		for _, p := range players {
			delete(t.inMatch, p.Id)
		}
	}

	return nil
}

func compareMatchEndTimes(a, b *got.Match) int {
	// Sort by most recently ended
	return b.EndTime.Compare(a.EndTime)
}
