package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindCompetitionHooks(app core.App) {
	url := "/competitions"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.DELETE("", deleteCompetitions).
			Bind(bodyIdList[Competition]("competitions"))
		return e.Next()
	})
}

func deleteCompetitions(e *core.RequestEvent) error {
	competitions := e.Get("competitions").([]*Competition)

	return tops.DeleteCompetitions(e.App, competitions)
}
