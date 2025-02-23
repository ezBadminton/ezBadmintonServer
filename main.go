package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ezBadminton/ezBadmintonServer/api"
	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"
	g "github.com/ezBadminton/ezBadmintonServer/generated"
	_ "github.com/ezBadminton/ezBadmintonServer/migrations"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	app := pocketbase.New()

	var watchClient bool
	app.RootCmd.PersistentFlags().BoolVar(
		&watchClient,
		"client-exit",
		false,
		"with this option the server terminates itself when the client exits. Only works when the server is a child process of the client.",
	)

	//RegisterHooks(app)
	//RegisterRoutes(app)
	api.InitRootApiRoute(app)
	api.BindClubHooks(app)
	api.BindRegistrationHooks(app)
	api.BindTournamentPlanHooks(app)
	api.BindDrawHooks(app)
	api.BindPlayerStatusHooks(app)
	api.BindMatchHooks(app)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}
		organizerCName := g.CName[g.TournamentOrganizer]()
		e.Router.GET(
			fmt.Sprintf("/api/ezbadminton/%s/exists", organizerCName),
			func(e *core.RequestEvent) error { return GetTournamentOrganizerExists(e, app) },
		)
		return e.Next()
	})

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		if err := store.InitStores(e.App); err != nil {
			return err
		}
		if err := tops.InitRegistrations(e.App); err != nil {
			return err
		}
		if err := tops.InitTournaments(e.App); err != nil {
			return err
		}
		if err := tops.InitCourts(); err != nil {
			return err
		}
		if err := tops.InitMatches(); err != nil {
			return err
		}

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// GetTournamentOrganizerExists handles GET requests to the /api/ezbadminton/tournament_organizer/exists route.
// It returns a JSON object with the "OrganizerUserExists" field telling wether a
// tournament organizer user is already registered or not.
func GetTournamentOrganizerExists(e *core.RequestEvent, dao core.App) error {
	exists := tournamentOrganizerExists()

	response := struct {
		OrganizerUserExists bool
	}{
		OrganizerUserExists: exists,
	}

	return e.JSON(http.StatusOK, response)
}

func tournamentOrganizerExists() bool {
	organizerCName := g.CName[g.TournamentOrganizer]()
	cache, _ := store.FindRecordStoreByCollectionName(organizerCName)
	return cache.Length() > 0
}
