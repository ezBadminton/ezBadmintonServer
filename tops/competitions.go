package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type CompetitionManager struct {
	// Before competition delete. After e.Next() the deletion has been persisted.
	onDelete *hook.Hook[*CompetitionEvent]
}

func newCompetitionManager() *CompetitionManager {
	return &CompetitionManager{
		onDelete: &hook.Hook[*CompetitionEvent]{},
	}
}

func (m *CompetitionManager) deleteCompetitions(app core.App, competitions []*Competition) error {
	return app.RunInTransaction(func(txApp core.App) error {
		for _, c := range competitions {
			event := newCompetitionEvent(txApp, c)
			err := m.onDelete.Trigger(event,
				m.cleanUpModeSettings,
				(*CompetitionEvent).saveDeletedCompetition,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *CompetitionManager) cleanUpModeSettings(e *CompetitionEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	settings := e.Competition.TournamentModeSettings()
	if settings == nil {
		return nil
	}
	parents := store.ListRelationParents(settings.Record)
	isOrphaned := len(parents) == 1

	if isOrphaned {
		e.Competition.SetTournamentModeSettings(nil)
		return e.App.Delete(settings)
	}
	return nil
}
