package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

type MatchManager struct {
	// Before match is started
	onStart *hook.Hook[*MatchEvent]
	// After match start time is set. After e.Next() the start time has been persisted.
	onAfterStart *hook.Hook[*MatchEvent]

	// Before match is canceled
	onCancel *hook.Hook[*MatchEvent]
	// After match start time is unset. After e.Next() the start time has been persisted.
	onAfterCancel *hook.Hook[*MatchEvent]

	// Before score data is set
	onScoreSet *hook.Hook[*ScoreEvent]
	// After score data is set. After e.Next() the score data has been persisted.
	onAfterScoreSet *hook.Hook[*ScoreEvent]

	// Before match reset
	onReset *hook.Hook[*ScoreEvent]
	// After match reset. After e.Next() the score data deletion has been persisted.
	onAfterReset *hook.Hook[*ScoreEvent]
}

func newMatchManager() *MatchManager {
	manager := &MatchManager{
		onStart:         &hook.Hook[*MatchEvent]{},
		onAfterStart:    &hook.Hook[*MatchEvent]{},
		onCancel:        &hook.Hook[*MatchEvent]{},
		onAfterCancel:   &hook.Hook[*MatchEvent]{},
		onScoreSet:      &hook.Hook[*ScoreEvent]{},
		onAfterScoreSet: &hook.Hook[*ScoreEvent]{},
		onReset:         &hook.Hook[*ScoreEvent]{},
		onAfterReset:    &hook.Hook[*ScoreEvent]{},
	}
	return manager
}

func (m *MatchManager) startMatch(app core.App, matchData *MatchData) error {
	event := newMatchEvent(app, matchData)
	return m.onStart.Trigger(event, m.startMatchHandler)
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus != Ready {
			return errors.New("the match is not in the ready state and can not be started")
		}
	*/

	/*
		match := m.tournamentStore.matches[matchData.Id]
		match.StartTime = startTime.Time()

		tournament := m.tournamentStore.byMatch[matchData.Id]
		tournament.UpdateEditableMatches()

		Scheduler.setMatchScheduleStatus(matchData, InProgress, nil)
	*/
}

func (m *MatchManager) startMatchHandler(e *MatchEvent) error {
	startTime := types.NowDateTime()
	e.MatchData.SetStartTime(startTime)

	if err := m.onAfterStart.Trigger(e, (*MatchEvent).saveMatchData); err != nil {
		return err
	}

	return e.Next()
}

func (m *MatchManager) cancelMatch(app core.App, matchData *MatchData) error {
	event := newMatchEvent(app, matchData)
	return m.onCancel.Trigger(event, m.cancelMatchHandler)
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus != InProgress {
			return errors.New("the match is not in progress and can not be canceled")
		}
	*/

	/*
		match := Tournaments.matches[matchData.Id]
		match.StartTime = time.Time{}

		tournament := Tournaments.byMatch[matchData.Id]
		tournament.UpdateEditableMatches()

		Scheduler.setMatchScheduleStatus(matchData, Ready, nil)
	*/
}

func (m *MatchManager) cancelMatchHandler(e *MatchEvent) error {
	matchData := Clone(e.MatchData)
	matchData.SetStartTime(types.DateTime{})
	e.MatchData = matchData

	if err := m.onAfterCancel.Trigger(e, (*MatchEvent).saveMatchData); err != nil {
		return err
	}
	return e.Next()
}

func (m *MatchManager) setMatchScore(app core.App, matchData *MatchData, points [][]int) error {
	event := NewScoreEvent(app, matchData)
	return m.onScoreSet.Trigger(event, func(e *ScoreEvent) error {
		return m.scoreSetHandler(e, points)
	})
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus == Done {
			if !isEditable(matchData) {
				return errors.New("the match is not editable")
			}
		} else if matchStatus != InProgress {
			return errors.New("the match is not in progress and can not have its score set")
		}
		// Set end time if inProgress
	*/

	/*
		if len(points) != 2 {
			return errors.New("invalid score")
		}

		tournament := m.tournamentStore.byMatch[matchData.Id]
		score, err := badminton.NewScore(points[0], points[1], tournament.ScoreSettings)
		if err != nil {
			return errors.New("invalid score")
		}
	*/

	/*
		match := m.tournamentStore.matches[matchData.Id]
		match.Score = score
		if matchStatus == InProgress {
			match.EndTime = endTime.Time()
			Scheduler.setMatchScheduleStatus(matchData, Done, matchData.Court())
			tournament.Ended = matchesFinished(tournament.MatchList().Matches)
		}

		Tournaments.update(tournament)
	*/
}

func (m *MatchManager) scoreSetHandler(e *ScoreEvent, points [][]int) error {
	scoreData := make([]*MatchSet, 0, 3)
	for i := range len(points[0]) {
		set, err := NewProxy[MatchSet](e.App)
		if err != nil {
			return err
		}
		set.SetTeam1Points(points[0][i])
		set.SetTeam2Points(points[1][i])
		scoreData = append(scoreData, set)
	}
	e.ScoreData = scoreData

	if err := m.onAfterScoreSet.Trigger(e, (*ScoreEvent).saveScoreData); err != nil {
		return err
	}

	return e.Next()
}

func (m *MatchManager) resetMatch(app core.App, matchData *MatchData) error {
	event := NewScoreEvent(app, matchData)
	return m.onReset.Trigger(event, m.resetMatchHandler)
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus != Done || !isEditable(matchData) {
			return errors.New("the match is in the wrong state to delete the score")
		}

		matchData = Clone(matchData)

		currentScoreData := matchData.Sets()
		currentCourt := matchData.Court()
		courtOccupied := Courts.isOccupied(currentCourt)
		if courtOccupied {
			matchData.SetCourt(nil)
		}
	*/

	/*
		match := Tournaments.matches[matchData.Id]

		match.Score = nil
		match.StartTime = time.Time{}
		match.EndTime = time.Time{}
		if courtOccupied {
			match.Location = nil
			Scheduler.setMatchScheduleStatus(matchData, CourtWait, nil)
		} else {
			Scheduler.setMatchScheduleStatus(matchData, Ready, currentCourt)
		}

		tournament := Tournaments.byMatch[matchData.Id]
		tournament.Ended = matchesFinished(tournament.MatchList().Matches)

		Tournaments.update(tournament)
	*/
}

func (m *MatchManager) resetMatchHandler(e *ScoreEvent) error {
	e.ScoreData = nil
	e.MatchData.SetStartTime(types.DateTime{})
	e.MatchData.SetEndTime(types.DateTime{})

	if err := m.onAfterReset.Trigger(e, (*ScoreEvent).saveScoreData); err != nil {
		return err
	}
	return e.Next()
}

/*
func isEditable(matchData *MatchData) bool {
	tournament := Tournaments.byMatch[matchData.Id]
	editable := tournament.EditableMatches()
	for _, m := range editable {
		editableMatchData := Tournaments.matchData[m.Id()]
		if editableMatchData.Id == matchData.Id {
			return true
		}
	}
	return false
}
*/
