package main

import (
	"log"

	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"

	_ "github.com/ezBadminton/ezBadmintonServer/migrations"

	"github.com/ezBadminton/ezBadmintonServer/hooks"

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
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}
		return e.Next()
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: false,
	})

	hooks.InitHooksAndApi(app)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
