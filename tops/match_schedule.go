package tops

import (
	"fmt"
	"iter"
	"slices"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
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
		"roundQueue": s.roundQueue,
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
}

var Scheduler *MatchScheduler

func InitScheduler(app core.App) {
	schedule := newSchedule()
	scheduled := scheduledMap(schedule)

	Scheduler = &MatchScheduler{
		app:       app,
		schedule:  schedule,
		scheduled: scheduled,
	}
}

func newSchedule() *Schedule {
	schedule := &Schedule{
		BaseTopsRecord: BaseTopsRecord{
			Id:      "the-schedule", // is a singleton
			Created: types.DateTime{},
			Updated: types.DateTime{},
		},
		roundQueue: make([]*ScheduledRound, 0),
	}

	runningTournaments := Tournaments.listStarted()

	if len(runningTournaments) == 0 {
		return schedule
	}

	offsets := calculateScheduleOffsets(runningTournaments)

	orderedRounds := make([][]*got.Match, 0)
	roundIndexes := make([]int, 0)
	roundCompetitions := make([]*Competition, 0)

	keepGoing := true
	for roundI := 0; keepGoing; roundI += 1 {
		keepGoing = false
		for tournI, tournament := range runningTournaments {
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
			matchData := Tournaments.matchData[m.Id()]
			matchStatus, blockingPlayers := scheduleStatus(m, competition)
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
		schedule.roundQueue = append(schedule.roundQueue, scheduledRound)
	}

	return schedule
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

func (s *MatchScheduler) scheduleStatus(matchData *MatchData) ScheduleStatus {
	return s.scheduled[matchData.Id].ScheduleStatus
}

func (s *MatchScheduler) setMatchScheduleStatus(matchData *MatchData, newStatus ScheduleStatus, court *Court) {
	scheduledMatch := s.scheduled[matchData.Id]
	currentStatus := scheduledMatch.ScheduleStatus

	scheduledMatch.ScheduleStatus = newStatus

	if occupationalState(currentStatus) && !occupationalState(newStatus) {
		delete(Courts.occupied, court.Id)
	} else if !occupationalState(currentStatus) && occupationalState(newStatus) {
		Courts.occupied[court.Id] = matchData.Id
	}

	go realtimeNotify(s.app, "scheduled_matches", core.ModelEventTypeUpdate, scheduledMatch)
}

func (s *MatchScheduler) updateTournamentScheduleStatus(tournament *CompetitionTournament) {
	competition := tournament.Competition
	matches := tournament.MatchList().Matches

	for _, m := range matches {
		matchData := Tournaments.matchData[m.Id()]
		status, blockingPlayers := scheduleStatus(m, competition)
		scheduledMatch := s.scheduled[matchData.Id]

		oldStatus := scheduledMatch.ScheduleStatus
		scheduledMatch.ScheduleStatus = status
		oldBlockingPlayers := scheduledMatch.BlockingPlayers
		scheduledMatch.BlockingPlayers = blockingPlayers

		if status != oldStatus || !blockingPlayersEq(blockingPlayers, oldBlockingPlayers) {
			go realtimeNotify(s.app, "scheduled_matches", core.ModelEventTypeUpdate, scheduledMatch)
		}
	}
}

func (s *MatchScheduler) tournamentStartStop(competition *Competition, started bool) {
	schedule := newSchedule()
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
			go realtimeNotify(s.app, "scheduled_matches", realtimeEventType, match)
		}
		go realtimeNotify(s.app, realtimeEventType, "scheduled_rounds", round)
	}
	go realtimeNotify(s.app, "schedule", core.ModelEventTypeUpdate, schedule)
}

func scheduleStatus(match *got.Match, competition *Competition) (ScheduleStatus, map[string]PlayerBlock) {
	if matchFinished(match) {
		return Done, nil
	}
	if matchStarted(match) {
		return InProgress, nil
	}
	if hasCourt(match) {
		return Ready, nil
	}

	players := playersInMatch(match)

	if len(players) < competition.TeamSize()*2 {
		return Wait, nil
	}

	blockingPlayers := make(map[string]PlayerBlock)

	playerWait, playerRest := false, false

	for _, p := range players {
		isPlaying, blockingMatch := PlayerTracker.isPlaying(p)
		if !isPlaying {
			continue
		}
		playerWait = true
		blockingPlayers[p.Id] = PlayerBlock{
			Mode:          Playing,
			BlockingMatch: blockingMatch,
		}
	}

	for _, p := range players {
		isResting, restUntil := PlayerTracker.isResting(p)
		if !isResting {
			continue
		}
		playerRest = true
		blockingPlayers[p.Id] = PlayerBlock{
			Mode:      Resting,
			RestUntil: restUntil,
		}
	}

	if playerWait {
		return PlayerWait, blockingPlayers
	}
	if playerRest {
		return PlayerRest, blockingPlayers
	}

	return CourtWait, nil
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
