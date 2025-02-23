package tops

import (
	"fmt"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
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

type PlayerScheduleStatus string

const (
	Playing PlayerScheduleStatus = "playing"
	Resting                      = "resting"
)

type ScheduledMatch struct {
	Match *MatchData
	ScheduleStatus
	PlayerStatus map[string]PlayerScheduleStatus
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

// The match scheduler holds an ordered list of
// rounds that represents the playing order.
type MatchScheduler struct {
	roundQueue     []*ScheduledRound
	runningMatches []*MatchData
	// Match data id -> scheduled match
	scheduled map[string]*ScheduledMatch
}

var Schedule *MatchScheduler

func InitSchedule() error {
	runningTournaments := Tournaments.listRunning()
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

	roundQueue := make([]*ScheduledRound, len(orderedRounds))
	runningMatches := make([]*MatchData, 0)
	scheduledMap := make(map[string]*ScheduledMatch)
	for i, round := range orderedRounds {
		scheduledMatches := make([]*ScheduledMatch, len(round))
		competition := roundCompetitions[i]
		roundIndex := roundIndexes[i]

		for i, m := range round {
			matchData := Tournaments.matchData[m.Id()]
			matchStatus, playerStatus := scheduleStatus(m, competition)
			scheduledMatches[i] = &ScheduledMatch{
				Match:          matchData,
				ScheduleStatus: matchStatus,
				PlayerStatus:   playerStatus,
			}
			if matchStatus == InProgress {
				runningMatches = append(runningMatches, matchData)
			}
			scheduledMap[matchData.Id] = scheduledMatches[i]
		}

		id := fmt.Sprintf("s-%v-%v", competition.Id, roundIndex)
		created := competition.Created()
		updated := competition.Updated()

		roundQueue[i] = &ScheduledRound{
			BaseTopsRecord: BaseTopsRecord{
				Id:      id,
				Created: created,
				Updated: updated,
			},
			Matches:     scheduledMatches,
			RoundIndex:  roundIndex,
			Competition: competition,
		}
	}

	Schedule = &MatchScheduler{
		roundQueue:     roundQueue,
		runningMatches: runningMatches,
		scheduled:      scheduledMap,
	}

	return nil
}

// TODO schedule updates

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
}

func (s *MatchScheduler) updateTournamentScheduleStatus(tournament *CompetitionTournament) {
	competition := tournament.Competition
	matches := tournament.MatchList().Matches

	for _, m := range matches {
		matchData := Tournaments.matchData[m.Id()]
		status, playerStatus := scheduleStatus(m, competition)
		scheduledMatch := s.scheduled[matchData.Id]

		scheduledMatch.ScheduleStatus = status
		scheduledMatch.PlayerStatus = playerStatus
	}
}

func scheduleStatus(match *got.Match, competition *Competition) (ScheduleStatus, map[string]PlayerScheduleStatus) {
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

	playerScheduleStatus := make(map[string]PlayerScheduleStatus)

	playerWait, playerRest := false, false

	for _, p := range players {
		isPlaying, _ := PlayerTracker.isPlaying(p)
		if !isPlaying {
			continue
		}
		playerWait = true
		playerScheduleStatus[p.Id] = Playing
	}

	for _, p := range players {
		isResting := PlayerTracker.isResting(p)
		if !isResting {
			continue
		}
		playerRest = true
		playerScheduleStatus[p.Id] = Resting
	}

	if playerWait {
		return PlayerWait, playerScheduleStatus
	}
	if playerRest {
		return PlayerRest, playerScheduleStatus
	}

	panic("something went wrong while determining match schedule status")
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
