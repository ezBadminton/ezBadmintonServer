package api

import (
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindTournamentPlanHooks(app core.App) {
	url := "/api/collections/tournamentplans"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("/records", listTournamentPlans)

		return e.Next()
	})
}

func listTournamentPlans(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListTournaments(), e)
}
