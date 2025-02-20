package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

type tournamentParticipant struct {
	*Player
}

func (p tournamentParticipant) Id() string {
	return p.Player.Id
}

func (c *CompetitionTournament) listWithdrawMatches(player *Player) []*MatchData {
	wrappedPlayer := tournamentParticipant{player}
	withdrawMatches := c.Tournament.ListWithdrawMatches(wrappedPlayer)
	return matchesToMatchData(withdrawMatches)
}

func (c *CompetitionTournament) listReenterMatches(player *Player) []*MatchData {
	wrappedPlayer := tournamentParticipant{player}
	reenterMatches := c.Tournament.ListReenterMatches(wrappedPlayer)
	return matchesToMatchData(reenterMatches)
}

func (c *CompetitionTournament) withdrawPlayer(player *Player) []*MatchData {
	wrappedPlayer := tournamentParticipant{player}
	withdrawMatches := c.Tournament.WithdrawPlayer(wrappedPlayer)
	return matchesToMatchData(withdrawMatches)
}

func (c *CompetitionTournament) reenterPlayer(player *Player) []*MatchData {
	wrappedPlayer := tournamentParticipant{player}
	reenterMatches := c.Tournament.ReenterPlayer(wrappedPlayer)
	return matchesToMatchData(reenterMatches)
}

func (c *CompetitionTournament) isWithdrawn(player *Player) bool {
	for _, m := range c.MatchList().Matches {
		players := playersInMatch(m)
		for _, p := range players {
			if p.Id == player.Id {
				return true
			}
		}
	}
	return false
}

type StatusChangeResult struct {
	Withdrawing bool
	Changes     map[*Competition][]*MatchData
}

func (r *StatusChangeResult) ToMap() map[string]any {
	changes := make(map[string]any, len(r.Changes))
	for comp, matches := range r.Changes {
		changes[comp.Id] = idList(matches)
	}
	return map[string]any{
		"withdrawing": r.Withdrawing,
		"changes":     changes,
	}
}

// When a player changes status, they withdraw or reenter from/to matches.
// This function returns the result of a status change without actually
// writing any changes
func listStatusChangeMatches(player *Player, newStatus PlayerStatus) *StatusChangeResult {
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
	changes := make(map[*Competition][]*MatchData)
	for _, reg := range regs {
		tournament := Tournaments.findTournament(reg.Competition.Id)
		if tournament == nil || !tournament.Started || tournament.Ended {
			continue
		}
		var changedMatches []*MatchData
		if withdrawing {
			changedMatches = tournament.listWithdrawMatches(player)
		} else {
			changedMatches = tournament.listReenterMatches(player)
		}

		addToChanges :=
			len(changedMatches) > 0 ||
				(!withdrawing && tournament.isWithdrawn(player))

		if addToChanges {
			changes[tournament.Competition] = changedMatches
		}
	}

	result := &StatusChangeResult{
		Withdrawing: withdrawing,
		Changes:     changes,
	}

	return result
}
