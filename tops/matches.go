package tops

import (
	"errors"

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
}

func (m *MatchManager) startMatchHandler(e *MatchEvent) error {
	e.MatchData.SetStartTime(types.NowDateTime())

	if err := m.onAfterStart.Trigger(e, (*MatchEvent).saveMatchData); err != nil {
		return err
	}
	return e.Next()
}

func (m *MatchManager) cancelMatch(app core.App, matchData *MatchData) error {
	event := newMatchEvent(app, matchData)
	return m.onCancel.Trigger(event, m.cancelMatchHandler)
}

func (m *MatchManager) cancelMatchHandler(e *MatchEvent) error {
	e.MatchData.SetStartTime(types.DateTime{})

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

	if err := m.onAfterScoreSet.Trigger(e, (*ScoreEvent).saveScoreData); err != nil {
		return err
	}

	return e.Next()
}

func (m *MatchManager) resetMatch(app core.App, matchData *MatchData) error {
	event := NewScoreEvent(app, matchData)
	return m.onReset.Trigger(event, m.resetMatchHandler)
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
