package main

import (
	"log"

	"github.com/ezBadminton/ezBadmintonServer/api"
	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"
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

	api.InitRootApiRoute(app)

	api.BindOrganizerRegistrationHooks(app)
	api.BindClubHooks(app)
	api.BindRegistrationHooks(app)
	api.BindTournamentPlanHooks(app)
	api.BindDrawHooks(app)
	api.BindPlayerStatusHooks(app)
	api.BindCourtHooks(app)
	api.BindMatchHooks(app)
	api.BindStartStopHooks(app)
	api.BindTieBreakerHooks(app)
	api.BindMatchScheduleHooks(app)
	api.BindEventSettingsHooks(app)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}
		return e.Next()
	})

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		if err := store.InitStores(e.App); err != nil {
			return err
		}
		tops.InitTournamentOperations(e.App)

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
