package api

import (
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindTournamentMatchHooks(app core.App) {
	url := "/api/collections/tournament_matches"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("/records", listTournamentMatches)

		return e.Next()
	})
}

func listTournamentMatches(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListMatches(), e)
}
