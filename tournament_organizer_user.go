package main

import (
	"errors"
	"net/http"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/daos"
)

// GetTournamentOrganizerExists handles GET requests to the /api/ezbadminton/tournament_organizer/exists route.
// It returns a JSON object with the "OrganizerUserExists" field telling wether a
// tournament organizer user is already registered or not.
func GetTournamentOrganizerExists(c echo.Context, dao *daos.Dao) error {
	exists, err := tournamentOrganizerExists(dao)

	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	response := struct {
		OrganizerUserExists bool
	}{
		OrganizerUserExists: exists,
	}

	return c.JSON(http.StatusOK, response)
}

func HandleBeforeTournamentOrganizerCreate(dao *daos.Dao) error {
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
func tournamentOrganizerExists(dao *daos.Dao) (bool, error) {
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
