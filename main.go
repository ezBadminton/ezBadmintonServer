package main

import (
	"log"

	watchers "github.com/ezBadminton/ezBadmintonServer/client_watchers"
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
		"the server terminates itself when the client exits. Only works when the server is a child process of the client.",
	)

	RegisterHooks(app)
	RegisterRoutes(app)

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		if watchClient {
			watchers.WatchClientForExit()
		}
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
