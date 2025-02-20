package tops

import (
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
)

func MakeDraw(comp *Competition) []*Team {
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

	draw := got.SeededShuffle(seeded, unseeded, seedingMode, rngSeed)
	return draw
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
