package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// Qualification overrides are manual qualification lists that
// take priority over the gotournament librariy's automatic
// qualification algorithm for group tournaments.
type QualificationOverrideManager struct {
	// Before qualification override update. After e.Next() the override has been persisted
	onUpdate *hook.Hook[*QualificationOverrideEvent]
	// After qualification override reset. After e.Next() the override has been deleted.
	onReset *hook.Hook[*QualificationOverrideEvent]
}

func newQualificationOverrideManager() *QualificationOverrideManager {
	return &QualificationOverrideManager{
		onUpdate: &hook.Hook[*QualificationOverrideEvent]{},
		onReset:  &hook.Hook[*QualificationOverrideEvent]{},
	}
}

func (m *QualificationOverrideManager) overrideSwap(app core.App, competition *Competition, a, b *Team) error {
	if a.Id == b.Id {
		return errors.New("team A and team B can not be the same")
	}

	return app.RunInTransaction(func(txApp core.App) error {
		event := newQualificationOverrideEvent(txApp, competition, a, b)
		err := m.onUpdate.Trigger(event,
			func(e *QualificationOverrideEvent) error {
				return m.overrideSwapHandler(e, a, b)
			},
			(*QualificationOverrideEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (m *QualificationOverrideManager) overrideSwapHandler(e *QualificationOverrideEvent, a, b *Team) error {
	override := slices.Clone(e.Competition.QualificationOverride())
	if len(override) == 0 {
		koEntries := e.Tournament.KnockOut.Entries.Ranks()
		override = make([]*Team, 0)
		for _, entry := range koEntries {
			override = append(override, entry.Player.(TournamentPlayer).Team)
		}
	}

	indexA := slices.IndexFunc(override, func(team *Team) bool { return team.Id == a.Id })
	indexB := slices.IndexFunc(override, func(team *Team) bool { return team.Id == b.Id })

	if indexA == -1 && indexB == -1 {
		return errors.New("none of the teams are in the current qualification list. can not swap")
	} else if indexA == -1 {
		override[indexB] = a
	} else if indexB == -1 {
		override[indexA] = b
	} else {
		override[indexA], override[indexB] = override[indexB], override[indexA]
	}

	e.QualificationOverride = override
	e.Competition.SetQualificationOverride(override)

	return e.Next()
}

func (m *QualificationOverrideManager) overrideReset(app core.App, competition *Competition) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newQualificationOverrideEvent(txApp, competition)
		err := m.onUpdate.Trigger(event,
			func(e *QualificationOverrideEvent) error {
				return m.overrideResetHandler(e)
			},
			(*QualificationOverrideEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (m *QualificationOverrideManager) overrideResetHandler(e *QualificationOverrideEvent) error {
	e.QualificationOverride = []*Team{}
	e.Competition.SetQualificationOverride(e.QualificationOverride)

	return e.Next()
}
