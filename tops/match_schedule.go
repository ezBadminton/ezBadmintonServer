package tops

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

type ScheduleStatus int

const (
	Done       ScheduleStatus = iota // Match finished
	InProgress                       // Currently running
	Ready                            // Court assigned, ready for call-out
	CourtWait                        // Match ready, waiting for court assignment
	PlayerRest                       // Match ready, but player rest time
	PlayerWait                       // Match ready, but player in other match
	Wait                             // Match not ready, waiting for qualifications
)

type PlayerBlockMode string

const (
	Playing PlayerBlockMode = "playing"
	Resting                 = "resting"
)

type PlayerBlock struct {
	Mode PlayerBlockMode

	// Only one of these is set depending on the block mode
	BlockingMatch *MatchData
	RestUntil     time.Time
}

func (b *PlayerBlock) ToMap() map[string]any {
	result := map[string]any{
		"mode": b.Mode,
	}
	if b.Mode == Playing {
		result["blockingMatch"] = b.BlockingMatch.Id
	} else {
		result["restUntil"] = b.RestUntil.UTC().Format(time.RFC3339)
	}
	return result
}

type ScheduledMatch struct {
	BaseTopsRecord
	Match *MatchData
	ScheduleStatus
	// player id -> block
	BlockingPlayers map[string]PlayerBlock
}

func (m *ScheduledMatch) ToMap() map[string]any {
	blockingPlayers := make(map[string]any, len(m.BlockingPlayers))
	for playerId, block := range m.BlockingPlayers {
		blockingPlayers[playerId] = block.ToMap()
	}
	result := map[string]any{
		"match":           m.Match.Id,
		"status":          m.ScheduleStatus,
		"blockingPlayers": blockingPlayers,
	}
	return m.BaseTopsRecord.ToMap(result)
}

// Equivalent to a Round in a MatchList except
// with meta info on the origin competition
type ScheduledRound struct {
	BaseTopsRecord
	Matches []*ScheduledMatch
	// Index of round in its tournament
	RoundIndex  int
	Competition *Competition
}

func (r *ScheduledRound) ToMap() map[string]any {
	matchIds := make([]string, len(r.Matches))
	for i, m := range r.Matches {
		matchIds[i] = m.Id
	}
	result := map[string]any{
		"matches":     matchIds,
		"roundIndex":  r.RoundIndex,
		"competition": r.Competition.Id,
	}
	return r.BaseTopsRecord.ToMap(result)
}

type Schedule struct {
	BaseTopsRecord
	roundQueue []*ScheduledRound
}

func (s *Schedule) ToMap() map[string]any {
	roundIds := make([]string, len(s.roundQueue))
	for i, round := range s.roundQueue {
		roundIds[i] = round.Id
	}
	result := map[string]any{
		"roundQueue": roundIds,
	}
	return s.BaseTopsRecord.ToMap(result)
}

func (s *Schedule) IterateMatches() iter.Seq[*ScheduledMatch] {
	return func(yield func(*ScheduledMatch) bool) {
		for _, round := range s.roundQueue {
			for _, match := range round.Matches {
				if !yield(match) {
					return
				}
			}
		}
	}
}

// The match scheduler holds an ordered list of
// rounds that represents the playing order.
type MatchScheduler struct {
	app      core.App
	schedule *Schedule
	// Match data id -> scheduled match
	scheduled map[string]*ScheduledMatch

	// Before a match schedule is (re-)made. After e.Next() the schedule is complete.
	onReschedule *hook.Hook[*RescheduleEvent]
	// Before a match's schedule status is determined. After e.Next() the status is determined.
	onStatusChange *hook.Hook[*ScheduleStatusEvent]
}

func newMatchScheduler(
	app core.App,
) *MatchScheduler {
	s := &MatchScheduler{
		app:            app,
		onReschedule:   &hook.Hook[*RescheduleEvent]{},
		onStatusChange: &hook.Hook[*ScheduleStatusEvent]{},
	}
	return s
}

func (s *MatchScheduler) init(
	tournamentStore *TournamentStore,
	courtStore *CourtStore,
	matchManager *MatchManager,
	playerTracker *PlayerTracker,
) {
	s.schedule = s.newSchedule()
	s.scheduled = scheduledMap(s.schedule)

	tournamentStore.onStart.BindFunc(s.handleTournamentStart)
	tournamentStore.onUpdate.BindFunc(s.handleTournamentPlanUpdate)
	tournamentStore.onStop.BindFunc(s.handleTournamentStop)

	courtStore.onCourtAssign.BindFunc(s.handleCourtAssignment)
	courtStore.onCourtUnassign.BindFunc(s.handleCourtUnassignment)

	matchManager.onStart.BindFunc(s.handleMatchStart)
	matchManager.onCancel.BindFunc(s.handleMatchCancel)
	matchManager.onScoreSet.BindFunc(s.handleScoreSet)
	matchManager.onReset.BindFunc(s.handleMatchReset)

	// Priority after TournamentStore
	playerTracker.onRestEnd.Bind(priorityHandler(s.handleRestEnd, 1))
	playerTracker.onRestChanged.Bind(priorityHandler(s.handleRestSettingsChange, 1))
}

func (s *MatchScheduler) newSchedule() *Schedule {
	schedule := &Schedule{
		BaseTopsRecord: BaseTopsRecord{
			Id:      "the-schedule", // is a singleton
			Created: types.NowDateTime(),
			Updated: types.NowDateTime(),
		},
		roundQueue: make([]*ScheduledRound, 0),
	}

	event := newRescheduleEvent(schedule)
	if err := s.onReschedule.Trigger(event, s.rescheduleHandler); err != nil {
		return nil
	}
	return event.Schedule
}

func (s *MatchScheduler) rescheduleHandler(e *RescheduleEvent) error {
	startedTournaments := e.StartedTournaments

	if len(startedTournaments) == 0 {
		return e.Next()
	}

	offsets := calculateScheduleOffsets(startedTournaments)

	orderedRounds := make([][]*got.Match, 0)
	roundIndexes := make([]int, 0)
	roundCompetitions := make([]*Competition, 0)

	keepGoing := true
	for roundI := 0; keepGoing; roundI += 1 {
		keepGoing = false
		for tournI, tournament := range startedTournaments {
			tournRoundI := roundI + offsets[tournI]
			if tournRoundI < 0 {
				continue
			}

			rounds := tournament.MatchList().Rounds
			if tournRoundI >= len(rounds) {
				continue
			}

			keepGoing = true

			roundMatches := rounds[tournRoundI].Matches
			orderedRounds = append(orderedRounds, roundMatches)
			roundIndexes = append(roundIndexes, tournRoundI)
			roundCompetitions = append(roundCompetitions, tournament.Competition)
		}
	}

	for i, round := range orderedRounds {
		scheduledMatches := make([]*ScheduledMatch, len(round))
		competition := roundCompetitions[i]
		roundIndex := roundIndexes[i]

		for i, m := range round {
			matchData, matchStatus, blockingPlayers := s.scheduleStatus(m, competition)
			id := "s-" + matchData.Id
			created := matchData.Created()
			updated := matchData.Updated()

			scheduledMatches[i] = &ScheduledMatch{
				BaseTopsRecord: BaseTopsRecord{
					Id:      id,
					Created: created,
					Updated: updated,
				},
				Match:           matchData,
				ScheduleStatus:  matchStatus,
				BlockingPlayers: blockingPlayers,
			}
		}

		id := fmt.Sprintf("s-%v-%v", competition.Id, roundIndex)
		created := competition.Created()
		updated := competition.Updated()

		scheduledRound := &ScheduledRound{
			BaseTopsRecord: BaseTopsRecord{
				Id:      id,
				Created: created,
				Updated: updated,
			},
			Matches:     scheduledMatches,
			RoundIndex:  roundIndex,
			Competition: competition,
		}
		e.Schedule.roundQueue = append(e.Schedule.roundQueue, scheduledRound)
	}

	return e.Next()
}

// match data id -> scheduled match
func scheduledMap(schedule *Schedule) map[string]*ScheduledMatch {
	scheduled := make(map[string]*ScheduledMatch)

	for match := range schedule.IterateMatches() {
		scheduled[match.Match.Id] = match
	}

	return scheduled
}

func (s *MatchScheduler) listSchedule() []*Schedule {
	return []*Schedule{s.schedule}
}

func (s *MatchScheduler) listScheduledRounds() []*ScheduledRound {
	return s.schedule.roundQueue
}

func (s *MatchScheduler) listScheduledMatches() []*ScheduledMatch {
	return slices.Collect(s.schedule.IterateMatches())
}

func (s *MatchScheduler) setMatchScheduleStatus(
	realtimeNotifier realtimeNotifier,
	matchData *MatchData,
	newStatus ScheduleStatus,
) {
	scheduledMatch := s.scheduled[matchData.Id]
	scheduledMatch.ScheduleStatus = newStatus

	realtimeNotifier.AddRealtimeNotification(s.app, "scheduled_matches", core.ModelEventTypeUpdate, scheduledMatch, 3)
}

func (s *MatchScheduler) triggerEventScheduleUpdates(e *MatchEvent, status ScheduleStatus) {
	override := map[int]statusOverride{
		e.Match.match.Id(): {status: status, matchData: e.MatchData},
	}
	for tournament, matches := range e.ScheduleUpdates {
		competition := tournament.Competition
		for m := range matches {
			s.updateMatchScheduleStatus(e, m.match, competition, override)
		}
	}
}

type statusOverride struct {
	status    ScheduleStatus
	matchData *MatchData
}

func (s *MatchScheduler) updateMatchScheduleStatus(
	realtimeNotifier realtimeNotifier,
	match *got.Match,
	competition *Competition,
	statusOverrides map[int]statusOverride,
) {
	var matchData *MatchData
	var status ScheduleStatus
	var blockingPlayers map[string]PlayerBlock

	override, ok := statusOverrides[match.Id()]
	if ok {
		matchData, status, blockingPlayers = override.matchData, override.status, nil
	} else {
		matchData, status, blockingPlayers = s.scheduleStatus(match, competition)
	}

	scheduledMatch := s.scheduled[matchData.Id]

	oldStatus := scheduledMatch.ScheduleStatus
	scheduledMatch.ScheduleStatus = status
	oldBlockingPlayers := scheduledMatch.BlockingPlayers
	scheduledMatch.BlockingPlayers = blockingPlayers

	if status != oldStatus || !blockingPlayersEq(blockingPlayers, oldBlockingPlayers) {
		realtimeNotifier.AddRealtimeNotification(s.app, "scheduled_matches", core.ModelEventTypeUpdate, scheduledMatch, 3)
	}
}

func (s *MatchScheduler) updateTournamentScheduleStatus(realtimeNotifier realtimeNotifier, tournament *CompetitionTournament) {
	competition := tournament.Competition
	matches := tournament.MatchList().Matches

	for _, m := range matches {
		s.updateMatchScheduleStatus(realtimeNotifier, m, competition, nil)
	}
}

func (s *MatchScheduler) tournamentStartStop(realtimeNotifier realtimeNotifier, competition *Competition, started bool) {
	schedule := s.newSchedule()
	s.schedule = schedule
	s.scheduled = scheduledMap(schedule)

	var realtimeEventType string
	if started {
		realtimeEventType = core.ModelEventTypeCreate
	} else {
		realtimeEventType = core.ModelEventTypeDelete
	}

	for _, round := range s.schedule.roundQueue {
		if round.Competition.Id != competition.Id {
			continue
		}
		for _, match := range round.Matches {
			realtimeNotifier.AddRealtimeNotification(s.app, "scheduled_matches", realtimeEventType, match, 3)
		}
		realtimeNotifier.AddRealtimeNotification(s.app, "scheduled_rounds", realtimeEventType, round, 4)
	}
	realtimeNotifier.AddRealtimeNotification(s.app, "schedule", core.ModelEventTypeUpdate, schedule, 5)
}

func (s *MatchScheduler) scheduleStatus(match *got.Match, competition *Competition) (*MatchData, ScheduleStatus, map[string]PlayerBlock) {
	event := newScheduleStatusEvent(match, competition)
	s.onStatusChange.Trigger(event, s.scheduleStatusHandler)
	return event.MatchData, event.Status, event.BlockingStatus
}

func (s *MatchScheduler) scheduleStatusHandler(e *ScheduleStatusEvent) error {
	match := e.Match
	if matchFinished(match) {
		e.Status = Done
		return e.Next()
	}
	if matchStarted(match) {
		e.Status = InProgress
		return e.Next()
	}
	if hasCourt(match) {
		e.Status = Ready
		return e.Next()
	}

	competition := e.Competition
	players := playersInMatch(match)

	if len(players) < competition.TeamSize()*2 {
		e.Status = Wait
		return e.Next()
	}

	blockingPlayers := make(map[string]PlayerBlock)

	playerWait, playerRest := false, false

	for p, blockingMatch := range e.PlayersInMatch {
		playerWait = true
		blockingPlayers[p.Id] = PlayerBlock{
			Mode:          Playing,
			BlockingMatch: blockingMatch,
		}
	}

	for p, restUntil := range e.PlayersResting {
		playerRest = true
		blockingPlayers[p.Id] = PlayerBlock{
			Mode:      Resting,
			RestUntil: restUntil,
		}
	}

	if playerWait {
		e.Status = PlayerWait
		e.BlockingStatus = blockingPlayers
		return e.Next()
	}
	if playerRest {
		e.Status = PlayerRest
		e.BlockingStatus = blockingPlayers
		return e.Next()
	}

	e.Status = CourtWait
	return e.Next()
}

func (s *MatchScheduler) handleTournamentStart(e *PlanEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	s.tournamentStartStop(e, e.Competition, true)
	return nil
}

func (s *MatchScheduler) handleTournamentPlanUpdate(e *PlanEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	s.updateTournamentScheduleStatus(e, e.Tournament)
	return nil
}

func (s *MatchScheduler) handleTournamentStop(e *PlanEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	s.tournamentStartStop(e, e.Competition, false)
	return nil
}

func (s *MatchScheduler) handleCourtAssignment(e *CourtEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != CourtWait {
		return errors.New("the match is not in the CourtWait status. Court can not be assigned.")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.triggerEventScheduleUpdates(e.MatchEvent, Ready)
	return nil
}

func (s *MatchScheduler) handleCourtUnassignment(e *CourtEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != Ready {
		return errors.New("the match is not in the Ready status. Court can not be unassigned.")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.triggerEventScheduleUpdates(e.MatchEvent, CourtWait)
	return nil
}

func (s *MatchScheduler) handleMatchStart(e *MatchEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != Ready {
		return errors.New("the match is not in the ready state and can not be started")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.setMatchScheduleStatus(e, e.MatchData, InProgress)
	return nil
}

func (s *MatchScheduler) handleMatchCancel(e *MatchEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != InProgress {
		return errors.New("the match is not in progress and can not be canceled")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.setMatchScheduleStatus(e, e.MatchData, Ready)
	return nil
}

func (s *MatchScheduler) handleScoreSet(e *ScoreEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != InProgress && matchStatus != Done {
		return errors.New("the match is not in progress or done and can not have its score set/edited")
	}

	if err := e.Next(); err != nil {
		return err
	}

	if matchStatus == InProgress {
		s.triggerEventScheduleUpdates(e.MatchEvent, Done)
	}
	return nil
}

func (s *MatchScheduler) handleMatchReset(e *ScoreEvent) error {
	matchStatus := s.scheduled[e.MatchData.Id].ScheduleStatus
	if matchStatus != Done {
		return errors.New("the match is not done and can not have its score reset")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.setMatchScheduleStatus(e, e.MatchData, CourtWait)
	return nil
}

func (s *MatchScheduler) handleRestEnd(e *PlayerRestEvent) error {
	for tournament, matches := range e.ScheduleUpdates {
		competition := tournament.Competition
		for m := range matches {
			s.updateMatchScheduleStatus(e, m.match, competition, nil)
		}
	}
	return e.Next()
}

func (s *MatchScheduler) handleRestSettingsChange(e *MatchRestEvent) error {
	for tournament, matches := range e.ScheduleUpdates {
		competition := tournament.Competition
		for m := range matches {
			s.updateMatchScheduleStatus(e, m.match, competition, nil)
		}
	}
	return e.Next()
}

// The schedule offset of a tournament is the amount
// of rounds that it is behind the tournament
// with the most rounds completed.
func calculateScheduleOffsets(tournaments []*CompetitionTournament) []int {
	completed := make([]int, len(tournaments))
	for i, t := range tournaments {
		completed[i] = numCompletedRounds(t)
	}
	maxCompleted := slices.Max(completed)

	offsets := make([]int, len(tournaments))
	for i, c := range completed {
		offset := c - maxCompleted
		offsets[i] = offset
	}

	return offsets
}

func numCompletedRounds(tournament got.MatchLister) int {
	completed := 0
	for _, round := range tournament.MatchList().Rounds {
		if !matchesFinished(round.Matches) {
			break
		}
		completed += 1
	}
	return completed
}

func blockingPlayersEq(a, b map[string]PlayerBlock) bool {
	if len(a) != len(b) {
		return false
	}
	for playerId, blockA := range a {
		blockB, ok := b[playerId]
		if !ok {
			return false
		}
		if blockA.Mode != blockB.Mode {
			return false
		}
		if blockA.Mode == Playing {
			if blockA.BlockingMatch.Id != blockA.BlockingMatch.Id {
				return false
			}
		} else {
			if blockA.RestUntil != blockB.RestUntil {
				return false
			}
		}
	}
	return true
}
