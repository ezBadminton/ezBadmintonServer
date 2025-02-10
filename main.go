package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ezBadminton/ezBadmintonServer/api"
	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"
	"github.com/ezBadminton/ezBadmintonServer/collection"
	_ "github.com/ezBadminton/ezBadmintonServer/migrations"

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
	api.BindPlayerHooks(app)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}
		e.Router.GET(
			fmt.Sprintf("/api/ezbadminton/%s/exists", collection.TournamentOrganizers),
			func(e *core.RequestEvent) error { return GetTournamentOrganizerExists(e, app) },
		)
		return e.Next()
	})

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		if err := collection.InitStores(e.App); err != nil {
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
	cache, _ := collection.FindRecordStoreByCollectionName(collection.TournamentOrganizers)
	return cache.Length() > 0
}
