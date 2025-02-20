package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

func (c *CompetitionTournament) listWithdrawMatches(team *Team) []*MatchData {
	wrapped := &TournamentPlayer{team}
	withdrawMatches := c.Tournament.ListWithdrawMatches(wrapped)
	return matchesToMatchData(withdrawMatches)
}

func (c *CompetitionTournament) listReenterMatches(team *Team) []*MatchData {
	wrapped := &TournamentPlayer{team}
	reenterMatches := c.Tournament.ListReenterMatches(wrapped)
	return matchesToMatchData(reenterMatches)
}

func (c *CompetitionTournament) withdrawTeam(team *Team) []*MatchData {
	wrapped := &TournamentPlayer{team}
	withdrawMatches := c.Tournament.WithdrawPlayer(wrapped)
	return matchesToMatchData(withdrawMatches)
}

func (c *CompetitionTournament) reenterTeam(team *Team) []*MatchData {
	wrapped := &TournamentPlayer{team}
	reenterMatches := c.Tournament.ReenterPlayer(wrapped)
	return matchesToMatchData(reenterMatches)
}

func (c *CompetitionTournament) isWithdrawn(team *Team) bool {
	for _, m := range c.MatchList().Matches {
		withdrawnTeams := m.WithdrawnPlayers
		for _, p := range withdrawnTeams {
			if p.Id() == team.Id {
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
		team := reg.Team
		if tournament == nil || !tournament.Started || tournament.Ended {
			continue
		}
		var changedMatches []*MatchData
		if withdrawing {
			changedMatches = tournament.listWithdrawMatches(team)
		} else {
			changedMatches = tournament.listReenterMatches(team)
		}

		addToChanges :=
			len(changedMatches) > 0 ||
				(!withdrawing && tournament.isWithdrawn(team))

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
