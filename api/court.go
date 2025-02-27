package api

import (
	"errors"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindCourtHooks(app core.App) {
	url := "/courts"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		dataFetcher := pathId[MatchData]("matchdata")
		group.POST("/{matchdata}/assign", assignCourt).Bind(dataFetcher)
		group.POST("/{matchdata}/unassign", unassignCourt).Bind(dataFetcher)

		return e.Next()
	})

	cName := CName[Court]()
	app.OnRecordDeleteRequest(cName).BindFunc(tops.DeleteCourt)
	cName = CName[Gymnasium]()
	app.OnRecordDeleteRequest(cName).BindFunc(tops.DeleteGymnasium)
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
	matchData := e.Get("matchdata").(*MatchData)

	if err := tops.UnassignCourt(e.App, matchData); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func readCourtAndMatchFromRequest(e *core.RequestEvent) (*Court, *MatchData, error) {
	matchData := e.Get("matchdata").(*MatchData)
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
	e.BindBody(&data)

	if data.CourtId == "" {
		// No court implies to the server to
		// select an open court automatically
		return nil, nil
	}

	courtId := data.CourtId
	court, err := store.FindProxy[Court](courtId)
	if err != nil {
		return nil, errors.New("court does not exist")
	}

	return court, nil
}
