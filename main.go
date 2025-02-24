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
	api.BindMatchHooks(app)

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
		if err := tops.InitRegistrations(e.App); err != nil {
			return err
		}
		if err := tops.InitTournaments(e.App); err != nil {
			return err
		}
		if err := tops.InitSchedule(); err != nil {
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
