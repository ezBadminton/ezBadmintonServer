package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindRegistrationHooks(app core.App) {
	url := "/registration"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/api/collections/registrations/records", listRegistrations).
			Bind(apis.RequireAuth())

		group := rootGroup.Group(url)

		group.POST("/{competition}", registerTeam).Bind(
			pathId[Competition]("competition"),
			bodyIdList[Player]("players"),
		)
		group.PATCH("/{team}", updateTeam).Bind(
			pathId[Team]("team"),
			bodyIdList[Player]("players"),
		)
		group.DELETE("/{team}", deleteTeam).
			Bind(pathId[Team]("team"))

		return e.Next()
	})
}

func listRegistrations(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListRegistrations(), e)
}

func registerTeam(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)
	players := e.Get("players").([]*Player)

	team, err := NewProxy[Team](e.App)
	if err != nil {
		return e.NoContent(http.StatusInternalServerError)
	}
	team.SetPlayers(players)

	if err := tops.RegisterTeam(e.App, team, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func updateTeam(e *core.RequestEvent) error {
	team := e.Get("team").(*Team)
	players := e.Get("players").([]*Player)

	team = Clone(team)
	team.SetPlayers(players)

	if err := tops.UpdateTeam(e.App, team); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func deleteTeam(e *core.RequestEvent) error {
	team := e.Get("team").(*Team)

	if err := tops.DeleteTeam(e.App, team); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}
