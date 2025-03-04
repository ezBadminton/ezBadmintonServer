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
			if err := txApp.Delete(c); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *CompetitionManager) deleteCompetition(re *core.RecordEvent) error {
	competition, _ := store.FindProxy[Competition](re.Record.Id)
	ce := newCompetitionEvent(re.App, competition)
	err := m.onDelete.Trigger(ce,
		m.cleanUpModeSettings,
		func(ce *CompetitionEvent) error {
			ce.syncParent(re)
			defer ce.syncToParent(re)
			return re.Next()
		},
	)
	ce.syncParent(re)
	return err
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
		return e.App.Delete(settings)
	}
	return nil
}
