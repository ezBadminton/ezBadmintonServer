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

	oldSettings := make([]*TournamentModeSettings, 0)
	oldSettingsCount := make(map[string]int)
	app := re.App
	err := re.App.RunInTransaction(func(txApp core.App) error {
		re.App = txApp
		if err := re.Next(); err != nil {
			return err
		}
		for _, c := range competitions {
			old := c.TournamentModeSettings()
			se := newTournamentModeSettingsEvent(re.App, settings, c)
			err := m.onSet.Trigger(se, m.settingsSetHandler)
			if err != nil {
				return err
			}
			if old != nil {
				oldSettings = append(oldSettings, old)
				oldSettingsCount[old.Id] = oldSettingsCount[old.Id] + 1
			}
		}
		if err := m.cleanUpOrphanedSettings(txApp, oldSettings, oldSettingsCount); err != nil {
			return err
		}

		return nil
	})
	re.App = app
	return err
}

func (m *TournamentModeSettingsManager) settingsSetHandler(e *TournamentModeSettingsEvent) error {
	e.Competition.SetTournamentModeSettings(e.Settings)

	if err := e.App.Save(e.Competition); err != nil {
		return err
	}
	return e.Next()
}

func (m *TournamentModeSettingsManager) cleanUpOrphanedSettings(
	app core.App,
	settings []*TournamentModeSettings,
	removeCounts map[string]int,
) error {
	for _, s := range settings {
		count, ok := removeCounts[s.Id]
		if !ok {
			continue
		}
		delete(removeCounts, s.Id)
		parents := store.ListRelationParents(s.Record)
		if len(parents)-count == 0 {
			if err := app.Delete(s); err != nil {
				return err
			}
		}
	}
	return nil
}
