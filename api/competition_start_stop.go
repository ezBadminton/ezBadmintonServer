package api

import (
	"errors"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindStartStopHooks(app core.App) {
	url := "/api/ezbadminton/startstop/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.POST("", handleStartStop)

		return e.Next()
	})
}

func handleStartStop(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	body := struct {
		Start bool `json:"start"`
	}{}
	if err := e.BindBody(&body); err != nil {
		return e.String(http.StatusBadRequest, "the body does not contain JSON with one 'start' bool field")
	}

	if body.Start {
		return startCompetition(e, competition)
	} else {
		return stopCompetition(e, competition)
	}
}

func startCompetition(e *core.RequestEvent, competition *Competition) error {
	err := tops.StartTournament(e.App, competition.Id)

	if errors.Is(err, tops.ErrUnexpected) {
		return e.NoContent(http.StatusInternalServerError)
	} else if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func stopCompetition(e *core.RequestEvent, competition *Competition) error {
	err := tops.StopTournament(e.App, competition.Id)

	if errors.Is(err, tops.ErrUnexpected) {
		return e.NoContent(http.StatusInternalServerError)
	} else if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}
