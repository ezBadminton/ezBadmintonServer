package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// A simple tuple of a Competition and a Team
// that indicates the team is registered in the
// competition.
type Registration struct {
	BaseTopsRecord
	*Competition
	*Team
	Withdrawn bool
}

func (r Registration) ToMap() map[string]any {
	data := map[string]any{
		"competition": r.Competition.Id,
		"team":        r.Team.Id,
		"withdrawn":   r.Withdrawn,
	}
	return r.BaseTopsRecord.ToMap(data)
}

type RegistrationStore struct {
	app                 core.App
	list                []*Registration
	byPlayer            map[string][]*Registration
	byTeam              map[string]*Registration
	byCompetition       map[string][]*Registration
	byCompetitionPlayer map[string]map[string]*Registration
	byId                map[string]*Registration

	// Before registration. After e.Next() the registration has been persisted.
	onRegistration *hook.Hook[*RegistrationEvent]
	// Before registration update. After e.Next() the registration update has been persisted.
	onUpdate *hook.Hook[*RegistrationEvent]
	// Before registration delete. After e.Next() the registration deletion has been persisted.
	// Is wrapped in a transaction.
	onDelete *hook.Hook[*RegistrationEvent]
}

func newRegistrationStore(
	app core.App,
	withdrawalManager *WithdrawalManager,
	competitionManager *CompetitionManager,
) *RegistrationStore {
	teamStore, _ := store.FindRecordStore[Team]()

	teams := teamStore.RecordList

	s := &RegistrationStore{
		app:                 app,
		list:                make([]*Registration, 0),
		byPlayer:            make(map[string][]*Registration),
		byTeam:              make(map[string]*Registration),
		byCompetition:       make(map[string][]*Registration),
		byCompetitionPlayer: make(map[string]map[string]*Registration),
		byId:                make(map[string]*Registration),
		onRegistration:      &hook.Hook[*RegistrationEvent]{},
		onUpdate:            &hook.Hook[*RegistrationEvent]{},
		onDelete:            &hook.Hook[*RegistrationEvent]{},
	}

	s.addTeams(teams...)

	// Priority for setting e.Registration
	withdrawalManager.onWithdraw.Bind(priorityHandler(s.verifyWithdrawal, -1))

	competitionManager.onDelete.BindFunc(s.handleCompetitonDeletion)

	return s
}

func (s *RegistrationStore) addTeams(teams ...*Team) {
	for _, team := range teams {
		reg := teamToRegistration(team)
		s.addRegistration(reg)
	}
}

func (s *RegistrationStore) registerTeam(app core.App, team *Team, competition *Competition) error {
	event := newRegistrationEvent(app, competition, team)
	event.AddedPlayers = team.Players()
	return s.onRegistration.Trigger(event,
		s.registerHandler,
		(*RegistrationEvent).saveNewRegistration,
		s.registerStoreHandler,
	)
}

func (s *RegistrationStore) registerHandler(e *RegistrationEvent) error {
	if err := s.verifyRegistration(e); err != nil {
		return err
	}

	reg := newRegistration(e.Team, e.Competition, false)
	e.Registration = reg

	if err := e.Next(); err != nil {
		return err
	}

	e.Registration.BaseTopsRecord.Created = e.Team.Created()
	e.Registration.BaseTopsRecord.Updated = e.Team.Updated()

	go realtimeNotify(e.App, "registrations", core.ModelEventTypeCreate, reg)
	return nil
}

// Stores the registration after it has been persisted
func (s *RegistrationStore) registerStoreHandler(e *RegistrationEvent) error {
	s.addRegistration(e.Registration)
	return e.Next()
}

func (s *RegistrationStore) updateTeam(app core.App, team *Team) error {
	event := newRegistrationEvent(app, nil, team)
	event.Registration = s.byTeam[team.Id]
	if event.Registration != nil {
		event.Competition = event.Registration.Competition
	}

	return s.onUpdate.Trigger(event,
		s.updateHandler,
		(*RegistrationEvent).saveUpdatedRegistration,
		s.updateStoreHandler,
	)
}

func (s *RegistrationStore) updateHandler(e *RegistrationEvent) error {
	team := e.Team
	reg := e.Registration
	if reg == nil {
		return errors.New("can not update unregistered team")
	}

	oldTeam, _ := store.FindProxy[Team](team.Id)
	oldPlayers := oldTeam.Players()

	updatedPlayers := team.Players()

	e.AddedPlayers = make([]*Player, 0, 2)
	e.RemovedPlayers = make([]*Player, 0, 2)

	for _, old := range oldPlayers {
		isGone := !slices.ContainsFunc(
			updatedPlayers,
			func(p *Player) bool { return p.Id == old.Id },
		)
		if isGone {
			e.RemovedPlayers = append(e.RemovedPlayers, old)
		}
	}
	for _, new := range updatedPlayers {
		isNew := !slices.ContainsFunc(
			oldPlayers,
			func(p *Player) bool { return p.Id == new.Id },
		)
		if isNew {
			e.AddedPlayers = append(e.AddedPlayers, new)
		}
	}

	if err := s.verifyRegistration(e); err != nil {
		return err
	}

	if err := e.Next(); err != nil {
		return err
	}

	go realtimeNotify(e.App, "registrations", core.ModelEventTypeUpdate, e.Registration)
	return nil
}

// Stores the updated registration after it has been persisted
func (s *RegistrationStore) updateStoreHandler(e *RegistrationEvent) error {
	e.Registration.Team = e.Team
	for _, p := range e.RemovedPlayers {
		s.playerRemoved(p, e.Team)
	}
	for _, p := range e.AddedPlayers {
		s.playerAdded(p, e.Team)
	}
	return e.Next()
}

func (s *RegistrationStore) deleteTeam(app core.App, team *Team) error {
	return app.RunInTransaction(func(txApp core.App) error {
		event := newRegistrationEvent(txApp, nil, team)
		event.Registration = s.byTeam[team.Id]
		if event.Registration != nil {
			event.Competition = event.Registration.Competition
		}
		return s.onDelete.Trigger(event,
			s.deleteHandler,
			(*RegistrationEvent).saveDeletedRegistration,
			s.deleteStoreHandler,
		)
	})
}

func (s *RegistrationStore) deleteHandler(e *RegistrationEvent) error {
	reg := e.Registration
	if reg == nil {
		return errors.New("can not delete unregistered team")
	}
	return e.Next()
}

// Updates the registration store after a delete
func (s *RegistrationStore) deleteStoreHandler(e *RegistrationEvent) error {
	reg := e.Registration
	comp := reg.Competition
	players := e.Team.Players()

	finder := func(r *Registration) bool { return r == reg }

	s.list = slices.DeleteFunc(s.list, finder)
	delete(s.byTeam, e.Team.Id)
	s.byCompetition[comp.Id] = slices.DeleteFunc(s.byCompetition[comp.Id], finder)
	for _, p := range players {
		s.byPlayer[p.Id] = slices.DeleteFunc(s.byPlayer[p.Id], finder)
		delete(s.byCompetitionPlayer[comp.Id], p.Id)
	}

	go realtimeNotify(e.App, "registrations", core.ModelEventTypeDelete, e.Registration)

	return e.Next()
}

func (s *RegistrationStore) verifyRegistration(e *RegistrationEvent) error {
	players := e.Team.Players()
	if len(players) == 0 {
		return errors.New("can not have a team without players")
	}
	if len(players) > e.Competition.TeamSize() {
		return errors.New("the team has too many players to be registered in this competition")
	}

	playerRegs, _ := s.byCompetitionPlayer[e.Competition.Id]
	for _, p := range e.AddedPlayers {
		_, ok := playerRegs[p.Id]
		if ok {
			return errors.New("a player of this team is already registered for this competition")
		}
	}

	return nil
}

func (s *RegistrationStore) addRegistration(reg *Registration) {
	comp := reg.Competition
	players := reg.Team.Players()

	s.list = append(s.list, reg)

	compPlayerRegs, ok := s.byCompetitionPlayer[comp.Id]
	if !ok {
		compPlayerRegs = make(map[string]*Registration)
		s.byCompetitionPlayer[comp.Id] = compPlayerRegs
	}
	for _, p := range players {
		playerRegs, ok := s.byPlayer[p.Id]
		if !ok {
			playerRegs = make([]*Registration, 0)
		}
		s.byPlayer[p.Id] = append(playerRegs, reg)
		compPlayerRegs[p.Id] = reg
	}

	compRegs, ok := s.byCompetition[comp.Id]
	if !ok {
		compRegs = make([]*Registration, 0)
	}
	s.byCompetition[comp.Id] = append(compRegs, reg)

	s.byTeam[reg.Team.Id] = reg
	s.byId[reg.Id] = reg
}

func (s *RegistrationStore) playerAdded(player *Player, team *Team) {
	reg := s.byTeam[team.Id]
	comp := reg.Competition

	playerRegs, ok := s.byPlayer[player.Id]
	if !ok {
		playerRegs = make([]*Registration, 0)
	}
	s.byPlayer[player.Id] = append(playerRegs, reg)

	s.byCompetitionPlayer[comp.Id][player.Id] = reg
}

func (s *RegistrationStore) playerRemoved(player *Player, team *Team) {
	reg := s.byTeam[team.Id]
	comp := reg.Competition

	s.byPlayer[player.Id] = slices.DeleteFunc(
		s.byPlayer[player.Id],
		func(r *Registration) bool { return r == reg },
	)

	delete(s.byCompetitionPlayer[comp.Id], player.Id)
}

func (s *RegistrationStore) handleCompetitonDeletion(ce *CompetitionEvent) error {
	if err := ce.Next(); err != nil {
		return err
	}

	registrations := ce.Competition.Registrations()
	for _, team := range registrations {
		re := newRegistrationEvent(ce.App, ce.Competition, team)
		re.Registration = s.byTeam[team.Id]

		err := s.onDelete.Trigger(re,
			s.deleteHandler,
			(*RegistrationEvent).saveDeletedRegistration,
			s.deleteStoreHandler,
			func(re *RegistrationEvent) error {
				re.syncParent(ce)
				return nil
			},
		)
		re.syncParent(ce)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *RegistrationStore) verifyWithdrawal(e *WithdrawEvent) error {
	reg, ok := s.byCompetitionPlayer[e.Competition.Id][e.StatusChangeEvent.Player.Id]
	if !ok {
		return errors.New("can not withdraw from competition where player is not registered")
	}
	e.Registration = reg
	return e.Next()
}

func newRegistration(team *Team, competition *Competition, withdrawn bool) *Registration {
	id := competition.Id + "-" + team.Id
	reg := &Registration{
		BaseTopsRecord: BaseTopsRecord{
			Id:      id,
			Created: team.Created(),
			Updated: team.Updated(),
		},
		Competition: competition,
		Team:        team,
		Withdrawn:   withdrawn,
	}
	return reg
}

func teamToRegistration(team *Team) *Registration {
	comp, withdrawn := findCompetitionOfTeam(team)
	return newRegistration(team, comp, withdrawn)
}

func findCompetitionOfTeam(t *Team) (*Competition, bool) {
	var competition *Competition
	var withdrawn bool
	relMap := store.RelationParentsByFieldName(t.Record)
	for parent, fieldNames := range relMap {
		parentProxy := store.FindRecord(parent)
		switch p := parentProxy.(type) {
		case *Competition:
			competition = p
		case *MatchData:
			if slices.Contains(fieldNames, "withdrawnTeams") {
				withdrawn = true
			}
		}
	}
	return competition, withdrawn
}
