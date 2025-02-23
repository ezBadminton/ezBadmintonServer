package tops

import (
	"errors"
	"math/rand"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

func makeDraw(app core.App, comp *Competition) error {
	if len(comp.Matches()) != 0 {
		return errors.New("can not make a draw for a running tournament")
	}

	draw := comp.Draw()
	comp, _ = WrapRecord[Competition](comp.Clone())

	registrations := comp.Registrations()
	attendingTeams := filterAttendingTeams(registrations)
	seeded := filterAttendingTeams(comp.Seeds())
	unseeded := make([]*Team, 0)

	for _, team := range attendingTeams {
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
		return nil
	}

	comp.SetDraw(newDraw)
	tournament, err := Tournaments.createTournament(comp)
	if err != nil {
		return err
	}

	if err := app.Save(comp); err != nil {
		return err
	}

	Tournaments.setTournament(comp, tournament)

	return nil
}

func redraw(app core.App, comp *Competition) error {
	comp, _ = WrapRecord[Competition](comp.Clone())
	comp.SetRngSeed(rand.Int())
	return makeDraw(app, comp)
}

func deleteDraw(app core.App, comp *Competition) error {
	if len(comp.Matches()) != 0 {
		return errors.New("can not make a draw change for a running tournament")
	}
	if len(comp.Draw()) == 0 {
		return errors.New("competition has no draw")
	}

	comp, _ = WrapRecord[Competition](comp.Clone())
	comp.SetDraw(nil)

	if err := app.Save(comp); err != nil {
		return err
	}

	Tournaments.removeTournament(comp)

	return nil
}

func drawSwap(app core.App, competition *Competition, a, b string) error {
	if len(competition.Matches()) != 0 {
		return errors.New("can not make a draw change for a running tournament")
	}

	draw := competition.Draw()
	if len(draw) == 0 {
		return errors.New("there is no draw to swap")
	}

	i0 := slices.IndexFunc(draw, idFinder[Team](a))
	i1 := slices.IndexFunc(draw, idFinder[Team](b))

	if i0 == -1 || i1 == -1 {
		return errors.New("the swapped IDs are not in the draw")
	}

	draw[i0], draw[i1] = draw[i1], draw[i0]

	competition, _ = WrapRecord[Competition](competition.Clone())
	competition.SetDraw(draw)

	if err := app.Save(competition); err != nil {
		return ErrUnexpected
	}

	tournament, _ := Tournaments.createTournament(competition)
	Tournaments.setTournament(competition, tournament)

	return nil
}

func setSeeds(app core.App, competition *Competition, seeds []*Team) error {
	if len(competition.Matches()) != 0 {
		return errors.New("can not set seeds for a running tournament")
	}
	registrations := competition.Registrations()
	if !containsAll(registrations, seeds) {
		return errors.New("can not seed unregistered team")
	}

	competition, _ = WrapRecord[Competition](competition.Clone())
	competition.SetSeeds(seeds)

	if err := app.Save(competition); err != nil {
		return err
	}

	return nil
}

func filterAttendingTeams(teams []*Team) []*Team {
	presentTeams := make([]*Team, 0, len(teams))
	for _, team := range teams {
		attending := true
		for _, player := range team.Players() {
			if player.Status() != Attending {
				attending = false
				break
			}
		}
		if attending {
			presentTeams = append(presentTeams, team)
		}
	}
	return presentTeams
}
