package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
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

var Registrations *RegistrationStore

type RegistrationStore struct {
	app                 core.App
	list                []*Registration
	byPlayer            map[string][]*Registration
	byTeam              map[string]*Registration
	byCompetition       map[string][]*Registration
	byCompetitionPlayer map[string]map[string]*Registration
	byId                map[string]*Registration
}

func InitRegistrations(app core.App) error {
	teamStore, err := store.FindRecordStore[Team]()
	if err != nil {
		return err
	}

	teams := teamStore.RecordList

	Registrations = &RegistrationStore{
		app:                 app,
		list:                make([]*Registration, 0),
		byPlayer:            make(map[string][]*Registration),
		byTeam:              make(map[string]*Registration),
		byCompetition:       make(map[string][]*Registration),
		byCompetitionPlayer: make(map[string]map[string]*Registration),
		byId:                make(map[string]*Registration),
	}

	Registrations.addTeams(teams...)

	return nil
}

func (s *RegistrationStore) addTeams(teams ...*Team) {
	for _, team := range teams {
		reg := teamToRegistration(team)
		s.addRegistration(reg)
	}
}

func (s *RegistrationStore) registerTeam(app core.App, team *Team, competition *Competition) error {
	if err := s.verifyRegistration(team, competition); err != nil {
		return err
	}

	err := app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(team); err != nil {
			return err
		}
		competition = Clone(competition)
		registrations := competition.Registrations()
		registrations = append(registrations, team)
		competition.SetRegistrations(registrations)
		if err := txApp.Save(competition); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	reg := newRegistration(team, competition, false)
	s.addRegistration(reg)

	return nil
}

func (s *RegistrationStore) updateTeam(app core.App, team *Team) error {
	reg := s.byTeam[team.Id]
	if reg == nil {
		return errors.New("can not update unregistered team")
	}
	if err := s.verifyRegistration(team, reg.Competition); err != nil {
		return err
	}

	oldTeam, _ := store.FindProxy[Team](team.Id)
	oldPlayers := oldTeam.Players()

	reg.Team = team
	updatedPlayers := team.Players()

	removedPlayers := make([]*Player, 0, 2)
	addedPlayers := make([]*Player, 0, 2)

	for _, old := range oldPlayers {
		isGone := !slices.ContainsFunc(
			updatedPlayers,
			func(p *Player) bool { return p.Id == old.Id },
		)
		if isGone {
			removedPlayers = append(removedPlayers, old)
		}
	}
	for _, new := range updatedPlayers {
		isNew := !slices.ContainsFunc(
			oldPlayers,
			func(p *Player) bool { return p.Id == new.Id },
		)
		if isNew {
			addedPlayers = append(addedPlayers, new)
		}
	}

	_, isActive := s.isRegistrationActive(reg)

	if len(removedPlayers) != len(addedPlayers) && isActive {
		return errors.New("can not add/remove team members while team is active in a tournament")
	}

	if err := app.Save(team); err != nil {
		return err
	}

	for _, p := range removedPlayers {
		s.playerRemoved(p, team)
	}
	for _, p := range addedPlayers {
		s.playerAdded(p, team)
	}

	return nil
}

func (s *RegistrationStore) deleteTeam(app core.App, team *Team) error {
	reg := s.byTeam[team.Id]
	if reg == nil {
		return errors.New("can not delete unregistered team")
	}

	comp := reg.Competition

	isInDraw, isActive := s.isRegistrationActive(reg)
	if isActive {
		return errors.New("can not delete team while it is active in a tournament")
	}

	err := app.RunInTransaction(func(txApp core.App) error {
		if isInDraw {
			if err := deleteDraw(txApp, comp); err != nil {
				return err
			}
		}

		return txApp.Delete(team)
	})
	if err != nil {
		return err
	}

	players := team.Players()

	finder := func(r *Registration) bool { return r == reg }

	s.list = slices.DeleteFunc(s.list, finder)
	delete(s.byTeam, team.Id)
	s.byCompetition[comp.Id] = slices.DeleteFunc(s.byCompetition[comp.Id], finder)
	for _, p := range players {
		s.byPlayer[p.Id] = slices.DeleteFunc(s.byPlayer[p.Id], finder)
		delete(s.byCompetitionPlayer[comp.Id], p.Id)
	}

	return nil
}

func (s *RegistrationStore) verifyRegistration(team *Team, competition *Competition) error {
	players := team.Players()
	if len(players) == 0 {
		return errors.New("can not have a team without players")
	}
	if len(players) > competition.TeamSize() {
		return errors.New("the team has too many players to be registered in this competition")
	}

	playerRegs, _ := s.byCompetitionPlayer[competition.Id]
	for _, p := range players {
		_, ok := playerRegs[p.Id]
		if ok {
			return errors.New("a player of this team is already registered for this competition")
		}
	}

	return nil
}

func (s *RegistrationStore) isRegistrationActive(reg *Registration) (bool, bool) {
	team := reg.Team
	comp := reg.Competition
	tournament := Tournaments.tournaments[comp.Id]

	isInDraw := slices.ContainsFunc(
		comp.Draw(),
		func(t *Team) bool { return t.Id == team.Id },
	)

	isActive := isInDraw && tournament.Started

	return isInDraw, isActive
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
