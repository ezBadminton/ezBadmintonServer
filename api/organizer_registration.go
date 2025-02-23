package api

import (
	"fmt"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

func BindOrganizerRegistrationHooks(app core.App) {
	organizerCName := CName[TournamentOrganizer]()
	url := fmt.Sprintf("/api/ezbadminton/%s/exists", organizerCName)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)

		group.GET("", tournamentOrganizerExists)

		return e.Next()
	})
}

func tournamentOrganizerExists(e *core.RequestEvent) error {
	store, err := store.FindRecordStore[TournamentOrganizer]()
	if err != nil {
		return e.NoContent(http.StatusInternalServerError)
	}
	exists := store.Length() > 0

	response := map[string]any{
		"OrganizerUserExists": exists,
	}

	return e.JSON(http.StatusOK, response)
}
