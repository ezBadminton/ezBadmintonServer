package tops

import (
	"errors"
	"math/rand"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type DrawManager struct {
	// Before draw being made. After e.Next() the draw has been persisted.
	onDraw *hook.Hook[*CompetitionEvent]
	// Before draw being deleted. After e.Next() the deletion has been persisted.
	onDrawDelete *hook.Hook[*CompetitionEvent]
	// Before draw positions being swapped. After e.Next() the swap has been persisted.
	onDrawSwap *hook.Hook[*CompetitionEvent]
	// Before seeds are being set. Adter e.Next() the seeds have been persisted.
	onSetSeeds *hook.Hook[*CompetitionEvent]
}

func newDrawManager(
	registrationStore *RegistrationStore,
	tournamentModeSettingsManager *TournamentModeSettingsManager,
) *DrawManager {
	m := &DrawManager{
		onDraw:       &hook.Hook[*CompetitionEvent]{},
		onDrawDelete: &hook.Hook[*CompetitionEvent]{},
		onDrawSwap:   &hook.Hook[*CompetitionEvent]{},
		onSetSeeds:   &hook.Hook[*CompetitionEvent]{},
	}

	registrationStore.onDelete.BindFunc(m.handleUnregistration)

	tournamentModeSettingsManager.onSet.BindFunc(m.handleModeSettingsUpdate)

	return m
}

func (d *DrawManager) makeDraw(app core.App, comp *Competition) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newCompetitionEvent(txApp, comp)
		err := d.onDraw.Trigger(event,
			d.makeDrawHandler,
			(*CompetitionEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (d *DrawManager) deleteDraw(app core.App, comp *Competition) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newCompetitionEvent(txApp, comp)
		err := d.onDrawDelete.Trigger(event,
			d.deleteDrawHandler,
			(*CompetitionEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (d *DrawManager) drawSwap(app core.App, competition *Competition, a, b string) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newCompetitionEvent(txApp, competition)
		err := d.onDrawSwap.Trigger(event,
			func(e *CompetitionEvent) error {
				return d.drawSwapHandler(e, a, b)
			},
			(*CompetitionEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (d *DrawManager) setSeeds(app core.App, competition *Competition, teams []*Team) error {
	event := newCompetitionEvent(app, competition)
	err := d.onSetSeeds.Trigger(event,
		func(e *CompetitionEvent) error {
			return d.setSeedsHandler(e, teams)
		},
		(*CompetitionEvent).saveCompetition,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
}

func (d *DrawManager) makeDrawHandler(e *CompetitionEvent) error {
	comp := e.Competition
	draw := comp.Draw()

	registrations := comp.Registrations()
	eligibleTeams := filterEligibleTeams(comp, registrations)
	seeded := filterEligibleTeams(comp, comp.Seeds())
	unseeded := make([]*Team, 0)

	for _, team := range eligibleTeams {
		isSeeded := slices.ContainsFunc(
			seeded,
			func(t *Team) bool { return t.Id == team.Id },
		)
		if !isSeeded {
			unseeded = append(unseeded, team)
		}
	}

	settings := comp.TournamentModeSettings()
	seedingMode := int(settings.SeedingMode())
	rngSeed := int64(comp.RngSeed())

	newDraw := got.SeededShuffle(seeded, unseeded, seedingMode, rngSeed)

	didChange := !slices.EqualFunc(
		draw, newDraw,
		func(a, b *Team) bool { return a.Id == b.Id },
	)
	if !didChange {
		return e.Next()
	}

	comp.SetDraw(newDraw)

	return e.Next()
}

func (d *DrawManager) deleteDrawHandler(e *CompetitionEvent) error {
	e.Competition.SetDraw(nil)
	return e.Next()
}

func (d *DrawManager) drawSwapHandler(e *CompetitionEvent, a, b string) error {
	draw := e.Competition.Draw()
	if len(draw) == 0 {
		return errors.New("there is no draw to swap")
	}

	i0 := slices.IndexFunc(draw, idFinder[Team](a))
	i1 := slices.IndexFunc(draw, idFinder[Team](b))

	if i0 == -1 || i1 == -1 {
		return errors.New("the swapped IDs are not in the draw")
	}

	draw[i0], draw[i1] = draw[i1], draw[i0]

	e.Competition.SetDraw(draw)

	return e.Next()
}

func (d *DrawManager) redraw(app core.App, comp *Competition) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newCompetitionEvent(txApp, comp)
		err := d.onDraw.Trigger(event,
			d.reseedHandler,
			d.makeDrawHandler,
			(*CompetitionEvent).saveCompetition,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
		return err
	})
}

func (d *DrawManager) reseedHandler(e *CompetitionEvent) error {
	e.Competition.SetRngSeed(rand.Int())
	return e.Next()
}

func (d *DrawManager) setSeedsHandler(e *CompetitionEvent, seeds []*Team) error {
	registrations := e.Competition.Registrations()
	if !containsAll(registrations, seeds) {
		return errors.New("can not seed unregistered team")
	}

	e.Competition.SetSeeds(seeds)

	return e.Next()
}

func (d *DrawManager) handleUnregistration(re *RegistrationEvent) error {
	if !isInDraw(re.Competition, re.Registration) {
		return re.Next()
	}

	app := re.App
	err := re.App.RunInTransaction(func(txApp core.App) error {
		re.App = txApp
		ce := newCompetitionEvent(re.App, re.Competition)
		err := d.onDrawDelete.Trigger(ce,
			d.deleteDrawHandler,
			(*CompetitionEvent).saveCompetition,
			func(ce *CompetitionEvent) error {
				ce.syncRegistrationParent(re)
				defer ce.syncToRegistrationParent(re)
				return re.Next()
			},
		)
		if err == nil {
			ce.TriggerRealtimeNotifications()
		}
		ce.syncRegistrationParent(re)
		return err
	})
	re.App = app
	return err
}

func (d *DrawManager) handleModeSettingsUpdate(se *TournamentModeSettingsEvent) error {
	if len(se.Competition.Draw()) == 0 {
		return se.Next()
	}
	ce := newCompetitionEvent(se.App, se.Competition)
	err := d.onDrawDelete.Trigger(ce,
		d.deleteDrawHandler,
		func(ce *CompetitionEvent) error {
			ce.syncSettingsParent(se)
			defer ce.syncToSettingsParent(se)
			return se.Next()
		},
	)
	if err == nil {
		ce.TriggerRealtimeNotifications()
	}
	ce.syncSettingsParent(se)
	return err
}

func filterEligibleTeams(competition *Competition, teams []*Team) []*Team {
	teamSize := competition.TeamSize()
	eligibleTeams := make([]*Team, 0, len(teams))
	for _, team := range teams {
		players := team.Players()
		eligible := len(players) == teamSize && allPlayersAttending(players)
		if eligible {
			eligibleTeams = append(eligibleTeams, team)
		}
	}
	return eligibleTeams
}

func allPlayersAttending(players []*Player) bool {
	for _, player := range players {
		if player.Status() != Attending {
			return false
		}
	}
	return true
}

func isInDraw(competition *Competition, reg *Registration) bool {
	team := reg.Team
	isInDraw := slices.ContainsFunc(
		competition.Draw(),
		func(t *Team) bool { return t.Id == team.Id },
	)
	return isInDraw
}
