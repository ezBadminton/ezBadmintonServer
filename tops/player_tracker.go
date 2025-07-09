package tops

import (
	"slices"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// A rest group is a group of players who have the same resting
// period because they played their last match together.
// Their rest timer can be canceled by writing to the cancel chan
type restGroup struct {
	cancel  chan any
	players []*Player
}

type PlayerTracker struct {
	// player id -> currently played match
	inMatch map[string]*TournamentMatch
	// player ids of resting players
	inRest map[string]any
	// player id -> last ended match
	lastMatches  map[string]*TournamentMatch
	restDuration time.Duration

	// match data id of recently ended match -> rest group
	restGroups map[string]restGroup

	cancelRestTimers chan any

	// When the rest period after a match ends the event informs about which players are not resting anymore
	onRestEnd *hook.Hook[*PlayerRestEvent]
	// When the rest duration setting changes the event informs about which matches need their rest period updated
	onRestChanged *hook.Hook[*MatchRestEvent]
}

func newPlayerTracker() *PlayerTracker {
	t := &PlayerTracker{
		inRest:           make(map[string]any),
		restGroups:       make(map[string]restGroup),
		cancelRestTimers: make(chan any),
		onRestEnd:        &hook.Hook[*PlayerRestEvent]{},
		onRestChanged:    &hook.Hook[*MatchRestEvent]{},
	}
	return t
}

func (t *PlayerTracker) init(
	tournamentStore *TournamentStore,
	courtStore *CourtStore,
	matchManager *MatchManager,
	eventSettingsManager *EventSettingsManager,
	scheduler *MatchScheduler,
) {
	tournamentEventStore, _ := store.FindRecordStore[TournamentEvent]()

	runningTournaments := tournamentStore.listStarted()
	matches := make([]*got.Match, 0)
	for _, t := range runningTournaments {
		matches = append(matches, t.MatchList().Matches...)
	}
	slices.SortFunc(matches, compareMatchEndTimes)

	tournamentEvent := tournamentEventStore.RecordList[0]
	restMinutes := tournamentEvent.PlayerRestTime()

	t.restDuration = time.Duration(restMinutes) * time.Minute
	t.lastMatches = collectLastMatches(matches, tournamentStore)
	t.inMatch = collectCurrentMatches(matches, tournamentStore)

	// Priority before schedule update
	courtStore.onCourtAssign.Bind(priorityHandler(t.handleCourtAssignment, 1))
	courtStore.onCourtUnassign.Bind(priorityHandler(t.handleCourtUnassignment, 1))

	// Priority before schedule update
	matchManager.onEnd.Bind(priorityHandler(t.handleMatchEnd, 1))
	matchManager.onScoreSet.Bind(priorityHandler(t.handleScoreSet, 1))
	matchManager.onReset.Bind(priorityHandler(t.prepareMatchReset, -3))
	matchManager.onReset.Bind(priorityHandler(t.handleMatchReset, 1))

	tournamentStore.onStop.BindFunc(t.handleTournamentStop)

	eventSettingsManager.onSettingsChange.BindFunc(t.handleRestTimeChange)

	// Priority for setting waiting and resting players in the event
	scheduler.onStatusChange.Bind(priorityHandler(t.handleScheduleStatus, -1))

	t.initRestTimers()
}

func collectCurrentMatches(matches []*got.Match, tournamentStore *TournamentStore) map[string]*TournamentMatch {
	inMatch := make(map[string]*TournamentMatch)
	for _, m := range matches {
		if !matchReady(m) && !matchRunning(m) {
			continue
		}

		matchData := tournamentStore.hydratedMatches[m.Id()]
		players := playersInMatch(m)

		for _, p := range players {
			inMatch[p.Id] = matchData
		}
	}
	return inMatch
}

// Goes through the sortedMatches (sorted by end time) and
// maps each player ID to their last ended match
func collectLastMatches(sortedMatches []*got.Match, tournamentStore *TournamentStore) map[string]*TournamentMatch {
	lastMatches := make(map[string]*TournamentMatch)
	for _, m := range sortedMatches {
		if m.EndTime.IsZero() {
			break
		}

		match := tournamentStore.hydratedMatches[m.Id()]
		players := playersInMatch(m)

		for _, p := range players {
			_, ok := lastMatches[p.Id]
			if !ok {
				lastMatches[p.Id] = match
			}
		}
	}
	return lastMatches
}

func (t *PlayerTracker) isPlaying(player *Player) (bool, *TournamentMatch) {
	match, ok := t.inMatch[player.Id]
	if !ok {
		return false, nil
	}
	return true, match
}

func (t *PlayerTracker) isResting(player *Player) (bool, time.Time) {
	_, ok := t.inRest[player.Id]
	if !ok {
		return false, time.Time{}
	}

	lastMatch := t.lastMatches[player.Id]
	restUntil := lastMatch.matchData.EndTime().Add(t.restDuration)

	return true, restUntil.Time()
}

func (t *PlayerTracker) startPlayerRest(players []*Player, match *TournamentMatch, duration time.Duration) {
	for _, p := range players {
		t.lastMatches[p.Id] = match
	}
	if duration <= 0 {
		return
	}
	for _, p := range players {
		t.inRest[p.Id] = struct{}{}
	}
	cancel := make(chan any)
	t.restGroups[match.Id] = restGroup{
		cancel:  cancel,
		players: players,
	}
	go t.restTimer(match, players, duration, cancel)
}

func (t *PlayerTracker) restTimer(match *TournamentMatch, players []*Player, duration time.Duration, cancel chan any) {
	timer := time.NewTimer(duration)
	select {
	case <-timer.C:
		t.handleRestEnd(match, players...)
	case <-cancel:
		timer.Stop()
	case _, _ = <-t.cancelRestTimers:
		timer.Stop()
	}
}

func (t *PlayerTracker) handleRestEnd(match *TournamentMatch, players ...*Player) {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	for _, p := range players {
		delete(t.inRest, p.Id)
	}
	_, ok := t.restGroups[match.Id]
	// ok would be false in case of the rest timer being
	// canceled before the lock for this function is acquired
	if ok {
		delete(t.restGroups, match.Id)
		event := newPlayerRestEvent(players, match)
		t.onRestEnd.Trigger(event)
		event.TriggerRealtimeNotifications()
	}
}

func (t *PlayerTracker) handleRestTimeChange(e *SettingsEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	newRestTime := e.NewSettings.PlayerRestTime()
	if e.OldSettings.PlayerRestTime() == newRestTime {
		return nil
	}
	t.restDuration = time.Duration(newRestTime) * time.Minute

	oldRestingMatches := make([]*TournamentMatch, 0)
	for playerId := range t.inRest {
		oldRestingMatches = append(oldRestingMatches, t.lastMatches[playerId])
	}
	t.inRest = make(map[string]any)
	t.restGroups = make(map[string]restGroup)
	close(t.cancelRestTimers)
	t.cancelRestTimers = make(chan any)

	newRestingMatches := t.initRestTimers()

	changedSet := make(map[string]*TournamentMatch)
	for _, m := range newRestingMatches {
		changedSet[m.Id] = m
	}
	for _, m := range oldRestingMatches {
		changedSet[m.Id] = m
	}

	changedRestingMatches := make([]*TournamentMatch, 0, len(changedSet))
	for _, m := range changedSet {
		changedRestingMatches = append(changedRestingMatches, m)
	}

	if len(changedSet) > 0 {
		event := newMatchRestEvent(changedRestingMatches)
		t.onRestChanged.Trigger(event)
		event.TriggerRealtimeNotifications()
	}

	return nil
}

// Starts all rest timers based on the current t.lastMatches
// Returns the last matches which had timers started for them.
func (t *PlayerTracker) initRestTimers() []*TournamentMatch {
	matchMap := make(map[*TournamentMatch][]*Player)
	for playerId, lastMatch := range t.lastMatches {
		_, ok := t.inMatch[playerId]
		if ok {
			continue
		}
		_, ok = matchMap[lastMatch]
		if !ok {
			matchMap[lastMatch] = make([]*Player, 0)
		}
		player, _ := store.FindProxy[Player](playerId)
		matchMap[lastMatch] = append(matchMap[lastMatch], player)
	}

	matchesInRest := make([]*TournamentMatch, 0)
	for lastMatch, players := range matchMap {
		restDuration := t.calculateRestDuration(lastMatch)
		if restDuration > 10*time.Second {
			matchesInRest = append(matchesInRest, lastMatch)
			t.startPlayerRest(players, lastMatch, restDuration)
		}
	}

	return matchesInRest
}

func (t *PlayerTracker) calculateRestDuration(match *TournamentMatch) time.Duration {
	restUntil := match.matchData.EndTime().Add(t.restDuration)
	restDuration := time.Until(restUntil.Time())
	return restDuration
}

func (t *PlayerTracker) startMatchPlayerRest(match *TournamentMatch) {
	players := playersInMatch(match.match)
	for _, p := range players {
		delete(t.inMatch, p.Id)
	}
	t.startPlayerRest(players, match, t.restDuration)
}

func (t *PlayerTracker) handleScheduleStatus(e *ScheduleStatusEvent) error {
	players := playersInMatch(e.Match)
	e.PlayersInMatch = make(map[*Player]*MatchData)
	e.PlayersResting = make(map[*Player]time.Time)
	for _, p := range players {
		isPlaying, blockingMatch := t.isPlaying(p)
		if isPlaying {
			e.PlayersInMatch[p] = blockingMatch.matchData
		}
		isResting, restUntil := t.isResting(p)
		if isResting {
			e.PlayersResting[p] = restUntil
		}
	}
	return e.Next()
}

func (t *PlayerTracker) handleCourtAssignment(e *CourtEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	players := playersInMatch(e.Match.match)
	for _, p := range players {
		t.inMatch[p.Id] = e.Match
	}
	return nil
}

func (t *PlayerTracker) handleCourtUnassignment(e *CourtEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	players := playersInMatch(e.Match.match)
	for _, p := range players {
		delete(t.inMatch, p.Id)
	}
	return nil
}

func (t *PlayerTracker) handleMatchEnd(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	t.startMatchPlayerRest(e.Match)
	return nil
}

func (t *PlayerTracker) handleScoreSet(e *ScoreEvent) error {
	matchEnding := e.MatchData.EndTime().IsZero()

	if err := e.Next(); err != nil {
		return err
	}

	if matchEnding {
		t.startMatchPlayerRest(e.Match)
	}
	return nil
}

func (t *PlayerTracker) prepareMatchReset(e *MatchResetEvent) error {
	players := playersInMatch(e.Match.match)
	for _, p := range players {
		if isPlaying, _ := t.isPlaying(p); isPlaying {
			e.CanStayOnCourt = false
			break
		}
		_, isResting := t.inRest[p.Id]
		if !isResting {
			continue
		}
		lastMatch := t.lastMatches[p.Id]
		if lastMatch.Id != e.Match.Id {
			e.CanStayOnCourt = false
			break
		}
	}
	return e.Next()
}

func (t *PlayerTracker) handleMatchReset(e *MatchResetEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	t.cancelRestGroup(e.MatchData)
	return nil
}

func (t *PlayerTracker) handleTournamentStop(e *PlanEvent) error {
	inMatchPlayers := make([]*Player, 0)
	matches := e.Tournament.MatchList().Matches
	for _, match := range matches {
		if match.Location == nil || !match.EndTime.IsZero() {
			continue
		}
		inMatchPlayers = append(inMatchPlayers, playersInMatch(match)...)
	}

	if err := e.Next(); err != nil {
		return err
	}

	for _, p := range inMatchPlayers {
		delete(t.inMatch, p.Id)
	}
	for _, matchData := range e.MatchData {
		if !matchData.EndTime().IsZero() {
			t.cancelRestGroup(matchData)
		}
	}
	return nil
}

func (t *PlayerTracker) cancelRestGroup(matchData *MatchData) {
	for playerId, lastMatch := range t.lastMatches {
		if lastMatch.Id == matchData.Id {
			delete(t.lastMatches, playerId)
		}
	}

	group, ok := t.restGroups[matchData.Id]
	if !ok {
		return
	}

	select {
	case group.cancel <- struct{}{}:
	default:
	}
	for _, p := range group.players {
		delete(t.inRest, p.Id)
	}

	delete(t.restGroups, matchData.Id)
}

func compareMatchEndTimes(a, b *got.Match) int {
	// Sort by most recently ended
	return b.EndTime.Compare(a.EndTime)
}
