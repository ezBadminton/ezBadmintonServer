package main

import (
	"log"

	_ "github.com/ezBadminton/ezBadmintonServer/migrations"

	"github.com/pocketbase/pocketbase"
)

func main() {
	app := pocketbase.New()

	RegisterHooks(app)
	RegisterRoutes(app)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
