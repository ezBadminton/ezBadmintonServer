package main

import (
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// HandleDeletedCompetition deletes the registered teams of a competition that is about to be deleted
func HandleDeletedCompetition(deletedCompetition *models.Record, dao *daos.Dao) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		if err := txDao.ExpandRecord(deletedCompetition, []string{registrationsName}, nil); len(err) != 0 {
			return err[registrationsName]
		}

		if err := DeleteRegistrationsOfCompetition(deletedCompetition, txDao); err != nil {
			return err
		}

		return nil
	})
}

// HandleCreatedTeam adds a newly created team to a competition's registrations list.
// Also deletes double registrations.
func HandleCreatedTeam(createdTeam *models.Record, competitionId string, dao *daos.Dao) error {
	if competitionId == "" {
		return nil
	}

	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		competition, err := txDao.FindRecordById(competitionsName, competitionId)
		if err != nil {
			return err
		}

		if err := deleteDoubleRegistrations(createdTeam, competition, txDao); err != nil {
			return err
		}

		newRegistrations := append(
			competition.GetStringSlice(registrationsName),
			createdTeam.Id,
		)

		competition.Set(registrationsName, newRegistrations)

		if err := txDao.SaveRecord(competition); err != nil {
			return err
		}

		return nil
	})
}

// HandleAfterUpdatedTeam checks if a team update caused the team to be empty
// and deletes it. Other updates are cascaded to the competition that the team
// is registered for.
func HandleAfterUpdatedTeam(updatedTeamModel models.Model, dao *daos.Dao) error {
	updatedTeam, fetchErr := dao.FindRecordById(teamsName, updatedTeamModel.GetId())

	if fetchErr != nil {
		return fetchErr
	}

	if len(updatedTeam.GetStringSlice(teamPlayersName)) == 0 {
		if err := dao.DeleteRecord(updatedTeam); err != nil {
			return err
		}
	} else {
		competitions, err := FindReverseMultiRelations(updatedTeam.Id, competitionsName, registrationsName, dao)
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
func HandleUpdatedTeam(updatedTeam *models.Record, dao *daos.Dao) error {
	teamSize := len(updatedTeam.GetStringSlice(teamPlayersName))

	// Empty teams get deleted anyways
	if teamSize == 0 {
		return nil
	}

	return dao.RunInTransaction(func(txDao *daos.Dao) error {
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

		competitionTeamSize := competition.GetInt(teamSizeName)

		if teamSize == competitionTeamSize {
			return nil
		}

		draw := competition.GetStringSlice(drawName)

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

		competition.Set(drawName, newDraw)

		if err := txDao.SaveRecord(competition); err != nil {
			return err
		}

		return nil

	})

}

// Deletes the given competition and the teams that are registered to it
func DeleteCompetitionAndTeams(competition *models.Record, dao *daos.Dao) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		if err := txDao.DeleteRecord(competition); err != nil {
			return err
		}

		if err := DeleteRegistrationsOfCompetition(competition, txDao); err != nil {
			return err
		}

		return nil
	})

}

func DeleteRegistrationsOfCompetition(competition *models.Record, dao *daos.Dao) error {
	registrations := competition.ExpandedAll(registrationsName)

	if err := ProcessRecords(registrations, dao.DeleteRecord); err != nil {
		return err
	}

	return nil
}

// Deletes teams that are registered to the competition and contain a member of the given team
func deleteDoubleRegistrations(team *models.Record, competition *models.Record, dao *daos.Dao) error {
	if err := dao.ExpandRecord(competition, []string{registrationsName}, nil); len(err) != 0 {
		return err[registrationsName]
	}
	registeredTeams := competition.ExpandedAll(registrationsName)

	doubleRegisteredTeams := findDoubleRegisteredTeams(registeredTeams, team)

	if err := ProcessRecords(doubleRegisteredTeams, dao.DeleteRecord); err != nil {
		return err
	}

	if len(doubleRegisteredTeams) != 0 {
		updatedCompetition, err := dao.FindRecordById(competitionsName, competition.Id)
		if err != nil {
			return err
		}
		*competition = *updatedCompetition
	}

	return nil
}

func findCompetitionOfTeam(teamId string, dao *daos.Dao) (*models.Record, error) {
	reverseRelations, err := FindReverseMultiRelations(teamId, competitionsName, registrationsName, dao)
	if err != nil {
		return nil, err
	}

	if len(reverseRelations) == 0 {
		return nil, nil
	}

	return reverseRelations[0], nil
}

func findDoubleRegisteredTeams(teams []*models.Record, newTeam *models.Record) []*models.Record {
	newTeamMemberIds := newTeam.GetStringSlice(teamPlayersName)

	doubleRegisteredTeams := make([]*models.Record, 0, 1)

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

func doTeamMembersOverlap(memberIds []string, team *models.Record) bool {
	for _, teamMemberId := range team.GetStringSlice(teamPlayersName) {
		for _, memberId := range memberIds {
			if teamMemberId == memberId {
				return true
			}
		}
	}

	return false
}
