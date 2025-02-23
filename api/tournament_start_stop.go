package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindStartStopHooks(app core.App) {
	url := "/api/ezbadminton/tournaments/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.POST("/start", startCompetition)
		group.POST("/stop", stopCompetition)

		return e.Next()
	})
}

func startCompetition(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	err = tops.StartTournament(e.App, competition.Id)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func stopCompetition(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	err = tops.StopTournament(e.App, competition.Id)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}
