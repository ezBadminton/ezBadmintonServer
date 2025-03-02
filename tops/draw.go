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
	// Before a draw is being made
	onDraw *hook.Hook[*CompetitionEvent]
	// After draw has been made. After e.Next() the draw has been persisted.
	onAfterDraw *hook.Hook[*CompetitionEvent]

	// Before draw is being deleted
	onDrawDelete *hook.Hook[*CompetitionEvent]
	// After draw has been deleted. After e.Next() the deletion has been persisted.
	onAfterDrawDelete *hook.Hook[*CompetitionEvent]

	// Before the draw positions are swapped
	onDrawSwap *hook.Hook[*CompetitionEvent]
	// After the swap has been made. After e.Next() the swap has been persisted.
	onAfterDrawSwap *hook.Hook[*CompetitionEvent]

	// Before seeds are set
	onSetSeeds *hook.Hook[*CompetitionEvent]
	// After seeds are set. Adter e.Next() the seeds have been persisted.
	onAfterSetSeeds *hook.Hook[*CompetitionEvent]
}

func newDrawManager(
	registrationStore *RegistrationStore,
	tournamentModeSettingsManager *TournamentModeSettingsManager,
) *DrawManager {
	m := &DrawManager{
		onDraw:            &hook.Hook[*CompetitionEvent]{},
		onAfterDraw:       &hook.Hook[*CompetitionEvent]{},
		onDrawDelete:      &hook.Hook[*CompetitionEvent]{},
		onAfterDrawDelete: &hook.Hook[*CompetitionEvent]{},
		onDrawSwap:        &hook.Hook[*CompetitionEvent]{},
		onAfterDrawSwap:   &hook.Hook[*CompetitionEvent]{},
		onSetSeeds:        &hook.Hook[*CompetitionEvent]{},
		onAfterSetSeeds:   &hook.Hook[*CompetitionEvent]{},
	}

	registrationStore.onAfterDelete.BindFunc(m.handleUnregistration)

	tournamentModeSettingsManager.onSet.BindFunc(m.handleModeSettingsUpdate)

	return m
}

func (d *DrawManager) makeDraw(app core.App, comp *Competition) error {
	event := newCompetitionEvent(app, comp)
	return d.onDraw.Trigger(event, d.makeDrawHandler)
}

func (d *DrawManager) deleteDraw(app core.App, comp *Competition) error {
	event := newCompetitionEvent(app, comp)
	return d.onDrawDelete.Trigger(event, d.deleteDrawHandler)
}

func (d *DrawManager) drawSwap(app core.App, competition *Competition, a, b string) error {
	event := newCompetitionEvent(app, competition)
	return d.onDrawSwap.Trigger(event, func(e *CompetitionEvent) error {
		return d.drawSwapHandler(e, a, b)
	})
}

func (d *DrawManager) setSeeds(app core.App, competition *Competition, teams []*Team) error {
	event := newCompetitionEvent(app, competition)
	return d.onSetSeeds.Trigger(event, func(e *CompetitionEvent) error {
		return d.setSeedsHandler(e, teams)
	})
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
			func(t *Team) bool { return t == team },
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

	if err := d.onAfterDraw.Trigger(e, (*CompetitionEvent).saveCompetition); err != nil {
		return err
	}
	return e.Next()
}

func (d *DrawManager) deleteDrawHandler(e *CompetitionEvent) error {
	e.Competition.SetDraw(nil)

	if err := d.onAfterDrawDelete.Trigger(e, (*CompetitionEvent).saveCompetition); err != nil {
		return err
	}
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

	if err := d.onAfterDrawSwap.Trigger(e, (*CompetitionEvent).saveCompetition); err != nil {
		return err
	}

	return e.Next()
}

func (d *DrawManager) redraw(app core.App, comp *Competition) error {
	event := newCompetitionEvent(app, comp)
	return d.onDraw.Trigger(event, d.redrawHandler)
}

func (d *DrawManager) redrawHandler(e *CompetitionEvent) error {
	e.Competition.SetRngSeed(rand.Int())
	return d.makeDrawHandler(e)
}

func (d *DrawManager) setSeedsHandler(e *CompetitionEvent, seeds []*Team) error {
	registrations := e.Competition.Registrations()
	if !containsAll(registrations, seeds) {
		return errors.New("can not seed unregistered team")
	}

	e.Competition.SetSeeds(seeds)

	if err := d.onAfterSetSeeds.Trigger(e, (*CompetitionEvent).saveCompetition); err != nil {
		return err
	}

	return e.Next()
}

func (d *DrawManager) handleUnregistration(e *RegistrationEvent) error {
	if isInDraw(e.Registration) {
		if err := d.deleteDraw(e.App, e.Competition); err != nil {
			return err
		}
	}
	return e.Next()
}

func (d *DrawManager) handleModeSettingsUpdate(e *TournamentModeSettingsEvent) error {
	if len(e.Competition.Draw()) == 0 {
		return e.Next()
	}
	return d.deleteDraw(e.App, e.Competition)
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

func isInDraw(reg *Registration) bool {
	team := reg.Team
	comp := reg.Competition
	isInDraw := slices.ContainsFunc(
		comp.Draw(),
		func(t *Team) bool { return t.Id == team.Id },
	)
	return isInDraw
}
