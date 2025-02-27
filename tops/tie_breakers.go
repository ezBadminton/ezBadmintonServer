package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type TieBreakerManager struct {
	// Before tie breaker add
	onAdd *hook.Hook[*TieBreakerEvent]
	// After tie breaker created. After e.Next() the tie breaker has been persisted
	onAfterAdd *hook.Hook[*TieBreakerEvent]

	// Before tie breaker update
	onUpdate *hook.Hook[*TieBreakerEvent]
	// After tie breaker updated. After e.Next() the tie breaker update has been persisted
	onAfterUpdate *hook.Hook[*TieBreakerEvent]

	// Before tie breaker delete. After e.Next() the tie breaker deletion has been persisted
	onDelete *hook.Hook[*TieBreakerEvent]
}

func newTieBreakerManager() *TieBreakerManager {
	return &TieBreakerManager{
		onAdd:         &hook.Hook[*TieBreakerEvent]{},
		onAfterAdd:    &hook.Hook[*TieBreakerEvent]{},
		onUpdate:      &hook.Hook[*TieBreakerEvent]{},
		onAfterUpdate: &hook.Hook[*TieBreakerEvent]{},
		onDelete:      &hook.Hook[*TieBreakerEvent]{},
	}
}

func (m *TieBreakerManager) addTieBreaker(app core.App, competition *Competition, teams []*Team) error {
	event := newTieBreakerEvent(app, competition, teams)
	return m.onAdd.Trigger(event, m.tieBreakerAddHandler)
}

func (m *TieBreakerManager) tieBreakerAddHandler(e *TieBreakerEvent) error {
	groupPhase := e.GroupPhase
	teams := e.Teams

	ties := make([][]*got.Slot, 0)
	for _, group := range groupPhase.Groups {
		numUntied := group.FinalRanking.RequiredUntiedRanks
		groupTies := group.FinalRanking.BlockingTies(numUntied)
		ties = append(ties, groupTies...)
	}
	crossTies := groupPhase.FinalRanking.CrossGroupTies()
	ties = append(ties, crossTies...)

	if err := verifyTieBreaker(ties, teams); err != nil {
		return err
	}

	tieBreaker, err := NewProxy[TieBreaker](e.App)
	if err != nil {
		return err
	}
	tieBreaker.SetTieBreakerRanking(teams)
	e.TieBreaker = tieBreaker

	if err := m.onAfterAdd.Trigger(e, (*TieBreakerEvent).saveNewTieBreaker); err != nil {
		return err
	}
	return e.Next()
}

func (m *TieBreakerManager) updateTieBreaker(app core.App, tieBreaker *TieBreaker, teams []*Team) error {
	competition, err := findCompetitionOfTieBreaker(tieBreaker)
	if err != nil {
		return err
	}
	event := newTieBreakerEvent(app, competition, teams)
	event.TieBreaker = tieBreaker
	return m.onUpdate.Trigger(event, m.tieBreakerUpdateHandler)
}

func (m *TieBreakerManager) tieBreakerUpdateHandler(e *TieBreakerEvent) error {
	tieBreaker := Clone(e.TieBreaker)
	tieBreakerTeams := tieBreaker.TieBreakerRanking()
	if len(tieBreakerTeams) != len(e.Teams) || !containsAll(tieBreakerTeams, e.Teams) {
		return errors.New("only update the order of a tie breaker, not the contained teams")
	}

	tieBreaker.SetTieBreakerRanking(e.Teams)
	e.TieBreaker = tieBreaker

	if err := m.onAfterUpdate.Trigger(e, (*TieBreakerEvent).saveUpdatedTieBreaker); err != nil {
		return err
	}
	return e.Next()
}

func (m *TieBreakerManager) deleteTieBreaker(app core.App, tieBreaker *TieBreaker) error {
	competition, err := findCompetitionOfTieBreaker(tieBreaker)
	if err != nil {
		return err
	}
	event := newTieBreakerEvent(app, competition, tieBreaker.TieBreakerRanking())
	event.TieBreaker = tieBreaker
	return m.onDelete.Trigger(event, (*TieBreakerEvent).saveDeletedTieBreaker)
}

func verifyTieBreaker(ties [][]*got.Slot, tieBreaker []*Team) error {
	var fits bool
	for _, tie := range ties {
		tiedTeams := make([]*Team, 0)
		for _, slot := range tie {
			if slot.Player == nil {
				continue
			}
			team := slot.Player.(TournamentPlayer).Team
			tiedTeams = append(tiedTeams, team)
		}
		if len(tieBreaker) == len(tiedTeams) && containsAll(tieBreaker, tiedTeams) {
			fits = true
			break
		}
	}
	if !fits {
		return errors.New("the tie breaker is not applicable to any of the present ties")
	}

	return nil
}

func findCompetitionOfTieBreaker(tieBreaker *TieBreaker) (*Competition, error) {
	parents := store.ListRelationParents(tieBreaker.Record)
	for _, p := range parents {
		comp, err := store.FindProxy[Competition](p.Id)
		if err == nil {
			return comp, nil
		}
	}
	return nil, errors.New("could not find competition of tie breaker")
}
