package main

import (
	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase/core"
)

// HandleDeletedCompetition deletes the registered teams of a competition that is about to be deleted
func HandleDeletedCompetition(deletedCompetition *core.Record, dao core.App) error {
	return dao.RunInTransaction(func(txDao core.App) error {
		if err := txDao.ExpandRecord(deletedCompetition, []string{names.Fields.Competitions.Registrations}, nil); len(err) != 0 {
			return err[names.Fields.Competitions.Registrations]
		}

		if err := DeleteRegistrationsOfCompetition(deletedCompetition, txDao); err != nil {
			return err
		}

		return nil
	})
}

// HandleCreatedTeam adds a newly created team to a competition's registrations list.
// Also deletes double registrations.
func HandleCreatedTeam(createdTeam *core.Record, competitionId string, dao core.App) error {
	if competitionId == "" {
		return nil
	}

	return dao.RunInTransaction(func(txDao core.App) error {
		competition, err := txDao.FindRecordById(names.Collections.Competitions, competitionId)
		if err != nil {
			return err
		}

		if err := deleteDoubleRegistrations(createdTeam, competition, txDao); err != nil {
			return err
		}

		newRegistrations := append(
			competition.GetStringSlice(names.Fields.Competitions.Registrations),
			createdTeam.Id,
		)

		competition.Set(names.Fields.Competitions.Registrations, newRegistrations)

		if err := txDao.Save(competition); err != nil {
			return err
		}

		return nil
	})
}

// HandleAfterUpdatedTeam checks if a team update caused the team to be empty
// and deletes it. Other updates are cascaded to the competition that the team
// is registered for.
func HandleAfterUpdatedTeam(updatedTeam *core.Record, dao core.App) error {
	if len(updatedTeam.GetStringSlice(names.Fields.Teams.Players)) == 0 {
		if err := dao.Delete(updatedTeam); err != nil {
			return err
		}
	} else {
		competitions, err := FindReverseMultiRelations(updatedTeam.Id, names.Collections.Competitions, names.Fields.Competitions.Registrations, dao)
		if err != nil {
			return err
		}

		if err := CascadeRelationUpdate(competitions, dao); err != nil {
			return err
		}
	}

	return nil
}

// HandleUpdatedTeam removes a team from its competition's draw when the
// update caused the team to have less members than the competition's team
// size requires.
// Also deletes double registrations that emerge from the update.
func HandleUpdatedTeam(updatedTeam *core.Record, dao core.App) error {
	teamSize := len(updatedTeam.GetStringSlice(names.Fields.Teams.Players))

	// Empty teams get deleted anyways
	if teamSize == 0 {
		return nil
	}

	return dao.RunInTransaction(func(txDao core.App) error {
		competition, err := findCompetitionOfTeam(updatedTeam.Id, txDao)
		if err != nil {
			return err
		}

		if competition == nil {
			return nil
		}

		if err := deleteDoubleRegistrations(updatedTeam, competition, txDao); err != nil {
			return err
		}

		competitionTeamSize := competition.GetInt(names.Fields.Competitions.TeamSize)

		if teamSize == competitionTeamSize {
			return nil
		}

		draw := competition.GetStringSlice(names.Fields.Competitions.Draw)

		indexOfTeam := -1

		for i, teamId := range draw {
			if teamId == updatedTeam.Id {
				indexOfTeam = i
				break
			}
		}

		if indexOfTeam == -1 {
			return nil
		}

		newDraw := append(draw[:indexOfTeam], draw[indexOfTeam+1:]...)

		competition.Set(names.Fields.Competitions.Draw, newDraw)

		if err := txDao.Save(competition); err != nil {
			return err
		}

		return nil

	})

}

// Deletes the given competition and the teams that are registered to it
func DeleteCompetitionAndTeams(competition *core.Record, dao core.App) error {
	return dao.RunInTransaction(func(txDao core.App) error {
		if err := txDao.Delete(competition); err != nil {
			return err
		}

		if err := DeleteRegistrationsOfCompetition(competition, txDao); err != nil {
			return err
		}

		return nil
	})

}

func DeleteRegistrationsOfCompetition(competition *core.Record, dao core.App) error {
	registrations := competition.ExpandedAll(names.Fields.Competitions.Registrations)

	if err := ProcessAsModels(registrations, dao.Delete); err != nil {
		return err
	}

	return nil
}

// Deletes teams that are registered to the competition and contain a member of the given team
func deleteDoubleRegistrations(team *core.Record, competition *core.Record, dao core.App) error {
	if err := dao.ExpandRecord(competition, []string{names.Fields.Competitions.Registrations}, nil); len(err) != 0 {
		return err[names.Fields.Competitions.Registrations]
	}
	registeredTeams := competition.ExpandedAll(names.Fields.Competitions.Registrations)

	doubleRegisteredTeams := findDoubleRegisteredTeams(registeredTeams, team)

	if err := ProcessAsModels(doubleRegisteredTeams, dao.Delete); err != nil {
		return err
	}

	if len(doubleRegisteredTeams) != 0 {
		updatedCompetition, err := dao.FindRecordById(names.Collections.Competitions, competition.Id)
		if err != nil {
			return err
		}
		*competition = *updatedCompetition
	}

	return nil
}

func findCompetitionOfTeam(teamId string, dao core.App) (*core.Record, error) {
	reverseRelations, err := FindReverseMultiRelations(teamId, names.Collections.Competitions, names.Fields.Competitions.Registrations, dao)
	if err != nil {
		return nil, err
	}

	if len(reverseRelations) == 0 {
		return nil, nil
	}

	return reverseRelations[0], nil
}

func findDoubleRegisteredTeams(teams []*core.Record, newTeam *core.Record) []*core.Record {
	newTeamMemberIds := newTeam.GetStringSlice(names.Fields.Teams.Players)

	doubleRegisteredTeams := make([]*core.Record, 0, 1)

	for _, team := range teams {
		if team.Id == newTeam.Id {
			continue
		}

		if doTeamMembersOverlap(newTeamMemberIds, team) {
			doubleRegisteredTeams = append(doubleRegisteredTeams, team)
		}
	}

	return doubleRegisteredTeams
}

func doTeamMembersOverlap(memberIds []string, team *core.Record) bool {
	for _, teamMemberId := range team.GetStringSlice(names.Fields.Teams.Players) {
		for _, memberId := range memberIds {
			if teamMemberId == memberId {
				return true
			}
		}
	}

	return false
}
