package main

import (
	"log"

	"github.com/ezBadminton/ezBadmintonServer/api"
	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"
	"github.com/ezBadminton/ezBadmintonServer/store"

	_ "github.com/ezBadminton/ezBadmintonServer/migrations"

	"github.com/ezBadminton/ezBadmintonServer/tops"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
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
	api.BindTournamentMatchHooks(app)
	api.BindDrawHooks(app)
	api.BindPlayerStatusHooks(app)
	api.BindCourtHooks(app)
	api.BindMatchHooks(app)
	api.BindStartStopHooks(app)
	api.BindTieBreakerHooks(app)
	api.BindMatchScheduleHooks(app)
	api.BindEventSettingsHooks(app)
	api.BindCategoryHooks(app)
	api.BindTournamentModeSettingsHooks(app)
	api.BindCompetitionHooks(app)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}

		if err := store.InitStores(e.App); err != nil {
			return err
		}
		tops.InitTournamentOperations(e.App)

		return e.Next()
	})

	app.Cron().Add("organizer_ping", "*/4 * * * *", func() {
		tops.PingOrganizerClients(app)
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: false,
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
