package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

type StatusChangeResult struct {
	Withdrawing bool                             `json:"withdrawn"`
	Changes     map[string]StatusChangeMatchList `json:"changes"`
}

type StatusChangeMatchList struct {
	Matches    []int `json:"matches"`
	CanReenter bool  `json:"canReenter"`
}

// When a player changes status, they withdraw or reenter from/to matches.
// This function lists the matches.
func listStatusChangedMatches(player *Player, newStatus PlayerStatus) error {
	var withdrawing bool
	status := player.Status()
	if status != Attending && newStatus == Attending {
		// Reentering
		withdrawing = false
	} else if status == Attending && newStatus != Attending {
		withdrawing = true
	} else {
		return nil
	}

	regs := Registrations.registrationsOfPlayer(player.Id)
	runningTournaments := make([]*CompetitionTournament, 0)
	for _, reg := range regs {
		tournament := Tournaments.findTournament(reg.Competition.Id)
		if tournament == nil {
			continue
		}

	}

	_, _ = withdrawing, runningTournaments

	return nil
}
