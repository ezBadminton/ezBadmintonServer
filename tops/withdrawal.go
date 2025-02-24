package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

func (c *CompetitionTournament) listWithdrawMatches(team *Team) []*MatchData {
	wrapped := TournamentPlayer{team}
	withdrawMatches := c.ListWithdrawMatches(wrapped)
	return matchesToMatchData(withdrawMatches)
}

func (c *CompetitionTournament) listReenterMatches(team *Team) []*MatchData {
	wrapped := TournamentPlayer{team}
	reenterMatches := c.ListReenterMatches(wrapped)
	return matchesToMatchData(reenterMatches)
}

type StatusChangeResult struct {
	Withdrawing bool
	Changes     map[*Competition][]*MatchData
}

func (r *StatusChangeResult) ToMap() map[string]any {
	if r == nil {
		return map[string]any{"changes": []string{}}
	}

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

	regs := Registrations.byPlayer[player.Id]
	changes := make(map[*Competition][]*MatchData)
	for _, reg := range regs {
		tournament, ok := Tournaments.tournaments[reg.Competition.Id]
		team := reg.Team
		if !ok || !tournament.Started || tournament.Ended {
			continue
		}
		var changedMatches []*MatchData
		if withdrawing {
			changedMatches = tournament.listWithdrawMatches(team)
		} else {
			changedMatches = tournament.listReenterMatches(team)
		}

		addToChanges := len(changedMatches) > 0 || (!withdrawing && reg.Withdrawn)

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

func setPlayerStatus(
	app core.App,
	player *Player,
	newStatus PlayerStatus,
	competitionIds []string,
) error {
	var withdraw bool
	var attendanceChanged bool
	currentStatus := player.Status()

	player = Clone(player)
	player.SetStatus(newStatus)

	if currentStatus == Attending && newStatus != Attending {
		attendanceChanged = true
		withdraw = true
	} else if currentStatus != Attending && newStatus == Attending {
		attendanceChanged = true
		withdraw = false
	}

	if !attendanceChanged && len(competitionIds) > 0 {
		return errors.New("can not withdraw/reenter from competitions with this status change")
	}

	changedMatches := make(map[*Registration][]*got.Match, 0)
	for _, id := range competitionIds {
		tournament, ok := Tournaments.tournaments[id]
		if !ok {
			return errors.New("can not withdraw from competition without draw")
		}
		reg, ok := Registrations.byCompetitionPlayer[id][player.Id]
		if !ok {
			return errors.New("can not withdraw from competition where player is not registered")
		}

		tPlayer := TournamentPlayer{reg.Team}
		var regChangedMatches []*got.Match
		if withdraw {
			regChangedMatches = tournament.ListWithdrawMatches(tPlayer)
		} else {
			regChangedMatches = tournament.ListReenterMatches(tPlayer)
		}
		if len(regChangedMatches) > 0 {
			changedMatches[reg] = regChangedMatches
		}
	}

	changedMatchData := make([]*MatchData, 0)
	for registration, matches := range changedMatches {
		updatedData := matchesToMatchData(matches)
		if withdraw {
			updatedData = addWithdrawnToData(registration.Team, updatedData)
		} else {
			updatedData = removeWithdrawnFromData(registration.Team, updatedData)
		}
		changedMatchData = append(changedMatchData, updatedData...)
	}

	err := app.RunInTransaction(func(txApp core.App) error {
		for _, m := range changedMatchData {
			if err := txApp.Save(m); err != nil {
				return err
			}
		}

		if err := txApp.Save(player); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	for registration, matches := range changedMatches {
		tPlayer := TournamentPlayer{registration.Team}
		if withdraw {
			addWithdrawnToMatches(tPlayer, matches)
		} else {
			removeWithdrawnFromMatches(tPlayer, matches)
		}

		tournament := Tournaments.tournaments[registration.Competition.Id]
		tournament.Update(nil)
		Schedule.updateTournamentScheduleStatus(tournament)
	}

	return nil
}

func addWithdrawnToData(team *Team, matchData []*MatchData) []*MatchData {
	changedMatchData := make([]*MatchData, len(matchData))
	for i, m := range matchData {
		changed := Clone(m)
		updatedWithdrawList := append(changed.WithdrawnTeams(), team)
		changed.SetWithdrawnTeams(updatedWithdrawList)
		changedMatchData[i] = changed
	}
	return changedMatchData
}

func removeWithdrawnFromData(team *Team, matchData []*MatchData) []*MatchData {
	changedMatchData := make([]*MatchData, len(matchData))
	for i, m := range matchData {
		changed := Clone(m)
		updatedWithdrawList := slices.DeleteFunc(
			changed.WithdrawnTeams(),
			func(t *Team) bool { return t.Id == team.Id },
		)
		changed.SetWithdrawnTeams(updatedWithdrawList)
		changedMatchData[i] = changed
	}
	return changedMatchData
}

func addWithdrawnToMatches(team TournamentPlayer, matches []*got.Match) {
	for _, m := range matches {
		m.WithdrawnPlayers = append(m.WithdrawnPlayers, team)
	}
}

func removeWithdrawnFromMatches(team TournamentPlayer, matches []*got.Match) {
	for _, m := range matches {
		updatedWithdrawList := slices.DeleteFunc(
			m.WithdrawnPlayers,
			func(t got.Player) bool { return t.Id() == team.Id() },
		)
		m.WithdrawnPlayers = updatedWithdrawList
	}
}
