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

func (m *EventSettingsManager) changeSettings(e *core.RecordRequestEvent) error {
	old, _ := store.FindProxy[TournamentEvent](e.Record.Id)
	new, _ := WrapRecord[TournamentEvent](e.Record)
	event := newSettingsEvent(e, old, new)
	return m.onSettingsChange.Trigger(event)
}
