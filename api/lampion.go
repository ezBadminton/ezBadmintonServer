package api

import (
	"net/http"

	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindLampionHooks(app core.App) {
	url := "/lampion"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		rootGroup.POST(url, importLampion)
		return e.Next()
	})
}

func importLampion(e *core.RequestEvent) error {
	if err := tops.ImportLampionTournament(); err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	return e.NoContent(http.StatusOK)
}
