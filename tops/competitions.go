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

func (m *CompetitionManager) deleteCompetition(re *core.RecordRequestEvent) error {
	competition, _ := store.FindProxy[Competition](re.Record.Id)
	app := re.App
	err := re.App.RunInTransaction(func(txApp core.App) error {
		re.App = txApp
		ce := newCompetitionEvent(re.App, competition)
		err := m.onDelete.Trigger(ce,
			func(ce *CompetitionEvent) error {
				ce.syncParent(re)
				defer ce.syncToParent(re)
				return re.Next()
			},
		)
		ce.syncParent(re)
		return err
	})
	re.App = app
	return err
}
