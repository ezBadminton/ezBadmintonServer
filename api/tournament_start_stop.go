package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindStartStopHooks(app core.App) {
	url := "/tournaments/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)
		group.Bind(pathId[Competition]("competition"))

		group.POST("/start", startCompetition)
		group.POST("/stop", stopCompetition)

		return e.Next()
	})
}

func startCompetition(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	err := tops.StartTournament(e.App, competition.Id)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func stopCompetition(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	err := tops.StopTournament(e.App, competition.Id)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}
