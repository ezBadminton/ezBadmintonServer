package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type TournamentModeSettingsManager struct {
	// Before new tourment mode settings are set.
	onSet *hook.Hook[*TournamentModeSettingsEvent]
}

func newTournamentModeSettingsManager() *TournamentModeSettingsManager {
	return &TournamentModeSettingsManager{
		onSet: &hook.Hook[*TournamentModeSettingsEvent]{},
	}
}

func (m *TournamentModeSettingsManager) setSettings(re *core.RecordRequestEvent) error {
	competitions, _ := re.RequestEvent.Get("competitions").([]*Competition)
	if len(competitions) == 0 {
		return errors.New("can not create tournament mode settings without specifying the target competitions")
	}
	settings, _ := WrapRecord[TournamentModeSettings](re.Record)

	app := re.App
	err := re.App.RunInTransaction(func(txApp core.App) error {
		re.App = txApp
		if err := re.Next(); err != nil {
			return err
		}
		for _, c := range competitions {
			se := newTournamentModeSettingsEvent(re.App, settings, c)
			err := m.onSet.Trigger(se, m.settingsSetHandler)
			if err != nil {
				return err
			}
		}
		return nil
	})
	re.App = app
	return err
}

func (m *TournamentModeSettingsManager) settingsSetHandler(e *TournamentModeSettingsEvent) error {
	oldSettings := e.Competition.TournamentModeSettings()
	var becomesUnused bool
	if oldSettings != nil {
		becomesUnused = len(store.ListRelationParents(oldSettings.Record)) == 1
	}

	e.Competition.SetTournamentModeSettings(e.Settings)

	if err := e.App.Save(e.Competition); err != nil {
		return err
	}

	if becomesUnused {
		if err := e.App.Delete(oldSettings); err != nil {
			return err
		}
	}

	return e.Next()
}
