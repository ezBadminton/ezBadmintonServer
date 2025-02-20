package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
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
	list                []*Registration
	byPlayer            map[string][]*Registration
	byTeam              map[string]*Registration
	byCompetition       map[string][]*Registration
	byCompetitionPlayer map[string]map[string]*Registration
	byId                map[string]*Registration
}

func InitRegistrations() error {
	teamStore, err := store.FindRecordStore[Team]()
	if err != nil {
		return err
	}

	teams := teamStore.RecordList

	Registrations = &RegistrationStore{
		list:                make([]*Registration, 0),
		byPlayer:            make(map[string][]*Registration),
		byTeam:              make(map[string]*Registration),
		byCompetition:       make(map[string][]*Registration),
		byCompetitionPlayer: make(map[string]map[string]*Registration),
		byId:                make(map[string]*Registration),
	}

	Registrations.addRegistrations(teams...)

	teamStore.RegisterCreateHander(Registrations.created)
	teamStore.RegisterUpdateHandler(Registrations.updated)
	teamStore.RegisterDeleteHandler(Registrations.deleted)

	return nil
}

func (s *RegistrationStore) registrationsOfPlayer(playerId string) []*Registration {
	return s.byPlayer[playerId]
}

func (s *RegistrationStore) verifyRegistration(team *Team, competition *Competition) error {
	if competition == nil {
		reg, ok := s.byTeam[team.Id]
		if !ok {
			return errors.New("the team is not registered yet")
		}
		competition = reg.Competition
	}

	players := team.Players()
	if len(players) > competition.TeamSize() {
		return errors.New("the team has too many players to be registered in this competition")
	}

	playerRegs, ok := s.byCompetitionPlayer[competition.Id]
	if !ok {
		return nil
	}

	for _, p := range players {
		_, ok := playerRegs[p.Id]
		if ok {
			return errors.New("a player of this team is already registered for this competition")
		}
	}

	return nil
}

func (s *RegistrationStore) addRegistrations(teams ...*Team) {
	for _, team := range teams {
		reg := teamToRegistration(team)
		comp := reg.Competition
		players := team.Players()

		s.list = append(s.list, reg)

		compPlayerRegs, ok := s.byCompetitionPlayer[comp.Id]
		if !ok {
			compPlayerRegs = make(map[string]*Registration)
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

		s.byTeam[team.Id] = reg
		s.byId[reg.Id] = reg
	}
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

func (s *RegistrationStore) created(team *Team) {
	Registrations.addRegistrations(team)
}

func (s *RegistrationStore) updated(oldTeam, updatedTeam *Team) {
	oldPlayers := oldTeam.Players()
	updatedPlayers := updatedTeam.Players()
	if len(oldPlayers) > len(updatedPlayers) {
		var removedPlayer *Player
		for _, removed := range oldPlayers {
			if !slices.ContainsFunc(updatedPlayers, func(p *Player) bool { return p.Id == removed.Id }) {
				removedPlayer = removed
				break
			}
		}
		s.playerRemoved(removedPlayer, updatedTeam)
	} else if len(updatedPlayers) > len(oldPlayers) {
		addedPlayer := updatedPlayers[len(updatedPlayers)-1]
		s.playerAdded(addedPlayer, updatedTeam)
	}
}

func (s *RegistrationStore) deleted(team *Team) {
	reg := s.byTeam[team.Id]
	comp := reg.Competition
	players := team.Players()

	finder := func(r *Registration) bool { return r == reg }

	s.list = slices.DeleteFunc(s.list, finder)

	delete(s.byTeam, team.Id)

	s.byCompetition[comp.Id] = slices.DeleteFunc(s.byCompetition[comp.Id], finder)

	for _, p := range players {
		s.byPlayer[p.Id] = slices.DeleteFunc(s.byPlayer[p.Id], finder)
		delete(s.byCompetitionPlayer[comp.Id], p.Id)
	}
}

func teamToRegistration(team *Team) *Registration {
	comp, withdrawn := findCompetitionOfTeam(team)
	id := comp.Id + "-" + team.Id
	reg := &Registration{
		BaseTopsRecord: BaseTopsRecord{
			Id:      id,
			Created: team.Created(),
			Updated: team.Updated(),
		},
		Competition: comp,
		Team:        team,
		Withdrawn:   withdrawn,
	}
	return reg
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
