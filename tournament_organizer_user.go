package main

import (
	"errors"
	"net/http"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase/core"
)

// GetTournamentOrganizerExists handles GET requests to the /api/ezbadminton/tournament_organizer/exists route.
// It returns a JSON object with the "OrganizerUserExists" field telling wether a
// tournament organizer user is already registered or not.
func GetTournamentOrganizerExists(e *core.RequestEvent, dao core.App) error {
	exists, err := tournamentOrganizerExists(dao)

	if err != nil {
		return e.NoContent(http.StatusInternalServerError)
	}

	response := struct {
		OrganizerUserExists bool
	}{
		OrganizerUserExists: exists,
	}

	return e.JSON(http.StatusOK, response)
}

func HandleBeforeTournamentOrganizerCreate(dao core.App) error {
	exists, err := tournamentOrganizerExists(dao)
	if err != nil {
		return err
	}

	if exists {
		return errors.New("cannot sign up more than one organizer")
	}

	return nil
}

// Returns wether a tournament organizer user exists
func tournamentOrganizerExists(dao core.App) (bool, error) {
	var count int

	err := dao.DB().NewQuery("SELECT COUNT(*) FROM " + names.Collections.TournamentOrganizer).Row(&count)
	if err != nil {
		return false, err
	}

	exists := true
	if count == 0 {
		exists = false
	}

	return exists, nil
}
