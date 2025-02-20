package tops

import (
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

func InitWithdrawalHandlers() error {
	playerStore, err := store.FindRecordStore[Player]()
	if err != nil {
		return err
	}

	playerStore.RegisterUpdateHandler(onPlayerStatusChange)
	playerStore.RegisterFailedUpdateHandler(onFailedPlayerStatusChange)

	return nil
}

type FloatingStatusChange struct {
	team     TournamentPlayer
	matches  []*got.Match
	withdraw bool
}

func (c *FloatingStatusChange) apply() {
	if c.withdraw {
		c.applyWithdrawal()
	} else {
		c.applyReenter()
	}
}

func (c *FloatingStatusChange) applyWithdrawal() {
	for _, m := range c.matches {
		m.WithdrawnPlayers = append(m.WithdrawnPlayers, c.team)
	}
}

func (c *FloatingStatusChange) applyReenter() {
	for _, m := range c.matches {
		m.WithdrawnPlayers = slices.DeleteFunc(
			m.WithdrawnPlayers,
			func(p got.Player) bool { return p.Id() == c.team.Id() },
		)
	}
}

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

func (c *CompetitionTournament) withdrawTeam(app core.App, team *Team) (*FloatingStatusChange, error) {
	wrapped := TournamentPlayer{team}
	withdrawMatches := c.ListWithdrawMatches(wrapped)
	matchData := matchesToMatchData(withdrawMatches)
	if err := saveWithdrawalLists(app, withdrawMatches, matchData); err != nil {
		return nil, err
	}
	change := &FloatingStatusChange{
		team:     wrapped,
		matches:  withdrawMatches,
		withdraw: true,
	}
	return change, nil
}

func (c *CompetitionTournament) reenterTeam(app core.App, team *Team) (*FloatingStatusChange, error) {
	wrapped := TournamentPlayer{team}
	reenterMatches := c.ListReenterMatches(wrapped)
	matchData := matchesToMatchData(reenterMatches)
	if err := saveWithdrawalLists(app, reenterMatches, matchData); err != nil {
		return nil, err
	}
	change := &FloatingStatusChange{
		team:     wrapped,
		matches:  reenterMatches,
		withdraw: false,
	}
	return change, nil
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

func saveWithdrawalLists(app core.App, matches []*got.Match, matchData []*MatchData) error {
	for i, m := range matches {
		withdrawn := make([]*Team, len(m.WithdrawnPlayers))
		for i, p := range m.WithdrawnPlayers {
			withdrawn[i] = p.(*TournamentPlayer).Team
		}
		data, _ := WrapRecord[MatchData](matchData[i].Clone())
		data.SetWithdrawnTeams(withdrawn)
		if err := app.Save(data); err != nil {
			return err
		}
	}
	return nil
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

func withdrawOrReenterPlayer(
	app core.App,
	player *Player,
	competitionIds []string,
	withdraw bool,
) ([]*FloatingStatusChange, error) {
	changes := make([]*FloatingStatusChange, 0)
	for _, id := range competitionIds {
		tournament := Tournaments.findTournament(id)
		if tournament == nil {
			continue
		}
		reg, ok := Registrations.byCompetitionPlayer[id][player.Id]
		if !ok {
			continue
		}
		var change *FloatingStatusChange
		var err error
		if withdraw {
			change, err = tournament.withdrawTeam(app, reg.Team)
		} else {
			change, err = tournament.reenterTeam(app, reg.Team)
		}
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func onPlayerStatusChange(_, player *Player) {
	customData := player.CustomData()
	statusChanges, ok := customData["STATUS_CHANGES"]
	if !ok {
		return
	}

	defer topsMu.Unlock()

	for _, change := range statusChanges.([]*FloatingStatusChange) {
		change.apply()
	}
}

func onFailedPlayerStatusChange(player *Player) {
	customData := player.CustomData()
	_, ok := customData["STATUS_CHANGES"]
	if ok {
		topsMu.Unlock()
	}
}
