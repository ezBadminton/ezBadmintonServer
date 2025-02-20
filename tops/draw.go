package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

func makeDraw(app core.App, comp *Competition) error {
	draw := comp.Draw()
	if len(draw) > 0 {
		return nil // Already has a draw
	}
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

	draw = got.SeededShuffle(seeded, unseeded, seedingMode, rngSeed)
	comp.SetDraw(draw)

	if err := app.Save(comp); err != nil {
		return errors.New("the competition is in the wrong state to have a draw made. there need to be enough players and valid tournament settings.")
	}

	return nil
}

func drawSwap(app core.App, competition *Competition, a, b string) error {
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
