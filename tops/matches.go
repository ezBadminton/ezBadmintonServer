package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

type MatchManager struct {
	// Before match is started. After e.Next() the start time has been persisted.
	onStart *hook.Hook[*MatchEvent]
	// Before match is canceled. After e.Next() the zeroed start time has been persisted.
	onCancel *hook.Hook[*MatchEvent]
	// Before score is set. After e.Next() the score data has been persisted.
	onScoreSet *hook.Hook[*ScoreEvent]
	// Before match reset. After e.Next() the score data deletion has been persisted.
	onReset *hook.Hook[*ScoreEvent]
}

func newMatchManager() *MatchManager {
	manager := &MatchManager{
		onStart:    &hook.Hook[*MatchEvent]{},
		onCancel:   &hook.Hook[*MatchEvent]{},
		onScoreSet: &hook.Hook[*ScoreEvent]{},
		onReset:    &hook.Hook[*ScoreEvent]{},
	}
	return manager
}

func (m *MatchManager) startMatch(app core.App, matchData *MatchData) error {
	event := newMatchEvent(app, matchData)
	return m.onStart.Trigger(event,
		m.startMatchHandler,
		(*MatchEvent).saveMatchData,
	)
}

func (m *MatchManager) startMatchHandler(e *MatchEvent) error {
	e.MatchData.SetStartTime(types.NowDateTime())
	return e.Next()
}

func (m *MatchManager) cancelMatch(app core.App, matchData *MatchData) error {
	event := newMatchEvent(app, matchData)
	return m.onCancel.Trigger(event,
		m.cancelMatchHandler,
		(*MatchEvent).saveMatchData,
	)
}

func (m *MatchManager) cancelMatchHandler(e *MatchEvent) error {
	e.MatchData.SetStartTime(types.DateTime{})
	return e.Next()
}

func (m *MatchManager) setMatchScore(app core.App, matchData *MatchData, points [][]int) error {
	event := NewScoreEvent(app, matchData)
	return m.onScoreSet.Trigger(event,
		func(e *ScoreEvent) error {
			return m.scoreSetHandler(e, points)
		},
		m.handleEndTimeSet,
		(*ScoreEvent).saveScoreData,
	)
}

func (m *MatchManager) scoreSetHandler(e *ScoreEvent, points [][]int) error {
	if len(points) != 2 || len(points[0]) != len(points[1]) {
		return errors.New("invalid points format")
	}
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

	return e.Next()
}

func (m *MatchManager) handleEndTimeSet(e *ScoreEvent) error {
	// Do not overwrite the end time on score edit
	if e.MatchData.EndTime().IsZero() {
		e.MatchData.SetEndTime(types.NowDateTime())
	}
	return e.Next()
}

func (m *MatchManager) resetMatch(app core.App, matchData *MatchData) error {
	event := NewScoreEvent(app, matchData)
	return m.onReset.Trigger(event,
		m.resetMatchHandler,
		(*ScoreEvent).saveScoreData,
	)
}

func (m *MatchManager) resetMatchHandler(e *ScoreEvent) error {
	e.ScoreData = nil
	e.MatchData.SetStartTime(types.DateTime{})
	e.MatchData.SetEndTime(types.DateTime{})
	return e.Next()
}
