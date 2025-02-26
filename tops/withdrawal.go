package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type WithdrawalManager struct {
	tournamentStore   *TournamentStore
	registrationStore *RegistrationStore

	// Before status change
	onStatusChange *hook.Hook[*StatusChangeEvent]
	// After withdrawals processed. After e.Next() the withdrawals and new status have been persisted
	onAfterStatusChange *hook.Hook[*StatusChangeEvent]

	// Before registration withdraw/reenter. After e.Next() the changed matches will be set
	onWithdraw *hook.Hook[*WithdrawEvent]
}

func newWithdrawalManager(tournamentStore *TournamentStore, registrationStore *RegistrationStore) *WithdrawalManager {
	return &WithdrawalManager{
		tournamentStore:     tournamentStore,
		registrationStore:   registrationStore,
		onStatusChange:      &hook.Hook[*StatusChangeEvent]{},
		onAfterStatusChange: &hook.Hook[*StatusChangeEvent]{},
		onWithdraw:          &hook.Hook[*WithdrawEvent]{},
	}
}

func (s *WithdrawalManager) listWithdrawMatches(team *Team, wp got.WithdrawalPolicy) []*MatchData {
	wrapped := TournamentPlayer{team}
	withdrawMatches := wp.ListWithdrawMatches(wrapped)
	return s.tournamentStore.matchesToMatchData(withdrawMatches)
}

func (s *WithdrawalManager) listReenterMatches(team *Team, wp got.WithdrawalPolicy) []*MatchData {
	wrapped := TournamentPlayer{team}
	reenterMatches := wp.ListReenterMatches(wrapped)
	return s.tournamentStore.matchesToMatchData(reenterMatches)
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
func (m *WithdrawalManager) listStatusChangeMatches(player *Player, newStatus PlayerStatus) *StatusChangeResult {
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

	regs := m.registrationStore.byPlayer[player.Id]
	changes := make(map[*Competition][]*MatchData)
	for _, reg := range regs {
		tournament, ok := m.tournamentStore.tournaments[reg.Competition.Id]
		team := reg.Team
		if !ok || !tournament.Started || tournament.Ended {
			continue
		}
		var changedMatches []*MatchData
		if withdrawing {
			changedMatches = m.listWithdrawMatches(team, tournament)
		} else {
			changedMatches = m.listReenterMatches(team, tournament)
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

func (m *WithdrawalManager) setPlayerStatus(
	app core.App,
	player *Player,
	newStatus PlayerStatus,
	competitions []*Competition,
) error {
	var withdraw bool
	var attendanceChanged bool
	currentStatus := player.Status()

	if currentStatus == Attending && newStatus != Attending {
		attendanceChanged = true
		withdraw = true
	} else if currentStatus != Attending && newStatus == Attending {
		attendanceChanged = true
		withdraw = false
	}

	if !attendanceChanged && len(competitions) > 0 {
		return errors.New("can not withdraw/reenter from competitions with this status change")
	}

	event := newStatusChangeEvent(app, player, newStatus, withdraw, competitions)
	return m.onStatusChange.Trigger(event, m.statusChangeHandler)
	/*
		for registration, matches := range changedMatches {
			tPlayer := TournamentPlayer{registration.Team}
			if withdraw {
				addWithdrawnToMatches(tPlayer, matches)
			} else {
				removeWithdrawnFromMatches(tPlayer, matches)
			}

			tournament := Tournaments.tournaments[registration.Competition.Id]
			Tournaments.update(tournament)
		}
	*/
}

func (m *WithdrawalManager) statusChangeHandler(e *StatusChangeEvent) error {
	for _, comp := range e.Competitions {
		/*
			tournament, ok := Tournaments.tournaments[comp.Id]
			if !ok || !tournament.Started || tournament.Ended {
				return errors.New("can not withdraw from competition that is not in progress")
			}
			reg, ok := Registrations.byCompetitionPlayer[comp.Id][player.Id]
			if !ok {
				return errors.New("can not withdraw from competition where player is not registered")
			}
			// Set tournament and reg in the event
		*/
		event := newWithdrawEvent(comp, e)
		if err := m.onWithdraw.Trigger(event, m.withdrawHandler); err != nil {
			return err
		}
		if len(event.ChangedMatches) > 0 {
			e.Withdrawals = append(e.Withdrawals, event)
		}
	}

	for _, withdrawal := range e.Withdrawals {
		updatedData := m.tournamentStore.matchesToMatchData(withdrawal.ChangedMatches)
		if e.Withdraw {
			updatedData = addWithdrawnToData(withdrawal.Registration.Team, updatedData)
		} else {
			updatedData = removeWithdrawnFromData(withdrawal.Registration.Team, updatedData)
		}
		e.ChangedMatchData = append(e.ChangedMatchData, updatedData...)
	}

	err := m.onAfterStatusChange.Trigger(e,
		(*StatusChangeEvent).saveStatusChange,
	)
	if err != nil {
		return err
	}
	return e.Next()
}

func (m *WithdrawalManager) withdrawHandler(e *WithdrawEvent) error {
	tPlayer := TournamentPlayer{e.Registration.Team}
	if e.StatusChangeEvent.Withdraw {
		e.ChangedMatches = e.WithdrawalPolicy.ListWithdrawMatches(tPlayer)
	} else {
		e.ChangedMatches = e.WithdrawalPolicy.ListReenterMatches(tPlayer)
	}
	return e.Next()
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
