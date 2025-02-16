package tops

import (
	"errors"
	"slices"
	"sync"

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
	List                []*Registration
	ByPlayer            map[string][]*Registration
	ByTeam              map[string]*Registration
	ByCompetition       map[string][]*Registration
	ByCompetitionPlayer map[string]map[string]*Registration
	ById                map[string]*Registration
	mu                  sync.RWMutex
}

func InitRegistrations() error {
	teamStore, err := store.FindRecordStore[Team]()
	if err != nil {
		return err
	}

	teams := teamStore.RecordList

	Registrations = &RegistrationStore{
		List:                make([]*Registration, 0),
		ByPlayer:            make(map[string][]*Registration),
		ByTeam:              make(map[string]*Registration),
		ByCompetition:       make(map[string][]*Registration),
		ByCompetitionPlayer: make(map[string]map[string]*Registration),
		ById:                make(map[string]*Registration),
	}

	Registrations.addRegistrations(teams...)

	teamStore.RegisterCreateHander(Registrations.Created)
	teamStore.RegisterUpdateHandler(Registrations.Updated)
	teamStore.RegisterDeleteHandler(Registrations.Deleted)

	return nil
}

func (s *RegistrationStore) VerifyRegistration(team *Team, competition *Competition) error {
	if competition == nil {
		reg, ok := s.ByTeam[team.Id]
		if !ok {
			return errors.New("the team is not registered yet")
		}
		competition = reg.Competition
	}

	players := team.Players()
	if len(players) > competition.TeamSize() {
		return errors.New("the team has too many players to be registered in this competition")
	}

	playerRegs, ok := s.ByCompetitionPlayer[competition.Id]
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
	defer s.mu.Unlock()
	s.mu.Lock()

	for _, team := range teams {
		reg := teamToRegistration(team)
		comp := reg.Competition
		players := team.Players()

		s.List = append(s.List, reg)

		compPlayerRegs, ok := s.ByCompetitionPlayer[comp.Id]
		if !ok {
			compPlayerRegs = make(map[string]*Registration)
		}

		for _, p := range players {
			playerRegs, ok := s.ByPlayer[p.Id]
			if !ok {
				playerRegs = make([]*Registration, 0)
			}
			s.ByPlayer[p.Id] = append(playerRegs, reg)
			compPlayerRegs[p.Id] = reg
		}

		compRegs, ok := s.ByCompetition[comp.Id]
		if !ok {
			compRegs = make([]*Registration, 0)
		}
		s.ByCompetition[comp.Id] = append(compRegs, reg)

		s.ByTeam[team.Id] = reg
		s.ById[reg.Id] = reg
	}
}

func (s *RegistrationStore) playerAdded(player *Player, team *Team) {
	defer s.mu.Unlock()
	s.mu.Lock()

	reg := s.ByTeam[team.Id]
	comp := reg.Competition

	playerRegs, ok := s.ByPlayer[player.Id]
	if !ok {
		playerRegs = make([]*Registration, 0)
	}
	s.ByPlayer[player.Id] = append(playerRegs, reg)

	s.ByCompetitionPlayer[comp.Id][player.Id] = reg
}

func (s *RegistrationStore) playerRemoved(player *Player, team *Team) {
	defer s.mu.Unlock()
	s.mu.Lock()

	reg := s.ByTeam[team.Id]
	comp := reg.Competition

	s.ByPlayer[player.Id] = slices.DeleteFunc(
		s.ByPlayer[player.Id],
		func(r *Registration) bool { return r == reg },
	)

	delete(s.ByCompetitionPlayer[comp.Id], player.Id)
}

func (s *RegistrationStore) Created(team *Team) {
	Registrations.addRegistrations(team)
}

func (s *RegistrationStore) Updated(oldTeam, updatedTeam *Team) {
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

func (s *RegistrationStore) Deleted(team *Team) {
	defer s.mu.Unlock()
	s.mu.Lock()

	reg := s.ByTeam[team.Id]
	comp := reg.Competition
	players := team.Players()

	finder := func(r *Registration) bool { return r == reg }

	s.List = slices.DeleteFunc(s.List, finder)

	delete(s.ByTeam, team.Id)

	s.ByCompetition[comp.Id] = slices.DeleteFunc(s.ByCompetition[comp.Id], finder)

	for _, p := range players {
		s.ByPlayer[p.Id] = slices.DeleteFunc(s.ByPlayer[p.Id], finder)
		delete(s.ByCompetitionPlayer[comp.Id], p.Id)
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
