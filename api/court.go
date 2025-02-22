package api

import (
	"errors"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindCourtHooks(app core.App) {
	url := "/api/ezbadminton/courts/{matchdata}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.POST("/assign", assignCourt)
		group.POST("/unassign", unassignCourt)

		return e.Next()
	})

	cName := CName[Court]()
	app.OnRecordDelete(cName).BindFunc(courtDelete)

	cName = CName[Gymnasium]()
	app.OnRecordDelete(cName).BindFunc(gymnasiumDelete)
}

func assignCourt(e *core.RequestEvent) error {
	court, matchData, err := readCourtAndMatchFromRequest(e)
	if err != nil {
		e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.AssignCourtToMatch(e.App, matchData, court); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func unassignCourt(e *core.RequestEvent) error {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.UnassignCourt(e.App, matchData); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func courtDelete(e *core.RecordEvent) error {
	court, _ := store.FindProxy[Court](e.Record.Id)

	if err := tops.VerifyCourtDeletion(court); err != nil {
		return err
	}
	return e.Next()
}

func gymnasiumDelete(e *core.RecordEvent) error {
	gym, _ := store.FindProxy[Gymnasium](e.Record.Id)
	if err := tops.VerifyGymnasiumDeletion(gym); err != nil {
		return err
	}

	courts := gym.CustomData()[tops.CourtsOfGymKey].([]*Court)

	app := e.App
	err := e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp

		for _, court := range courts {
			if err := e.App.Delete(court); err != nil {
				return err
			}
		}

		return e.Next()
	})
	e.App = app

	return err
}

func readCourtAndMatchFromRequest(e *core.RequestEvent) (*Court, *MatchData, error) {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return nil, nil, err
	}
	court, err := readCourtFromBody(e)
	if err != nil {
		return nil, nil, err
	}
	return court, matchData, nil
}

func readCourtFromBody(e *core.RequestEvent) (*Court, error) {
	data := struct {
		CourtId string `json:"court"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return nil, errors.New("body does not contain a JSON 'court' fied")
	}

	courtId := data.CourtId
	court, err := store.FindProxy[Court](courtId)
	if err != nil {
		return nil, errors.New("court does not exist")
	}

	return court, nil
}
