package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type EventSettingsManager struct {
	// Before settings change. After e.Next() the new settings have been persisted
	onSettingsChange *hook.Hook[*SettingsEvent]
}

func newEventSettingsManager() *EventSettingsManager {
	return &EventSettingsManager{
		onSettingsChange: &hook.Hook[*SettingsEvent]{},
	}
}

func (m *EventSettingsManager) changeSettings(re *core.RecordRequestEvent) error {
	old, _ := store.FindProxy[TournamentEvent](re.Record.Id)
	old = Clone(old)
	new, _ := WrapRecord[TournamentEvent](re.Record)

	se := newSettingsEvent(re, old, new)
	err := m.onSettingsChange.Trigger(se, func(se *SettingsEvent) error {
		se.syncRequest(re)
		defer se.syncToRequest(re)
		return re.Next()
	})
	se.syncRequest(re)
	return err
}
