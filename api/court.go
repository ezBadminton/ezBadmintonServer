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

		group.DELETE("/{court}", deleteCourt).
			Bind(pathId[Court]("court"))

		group.DELETE("/gymnasium/{gymnasium}", deleteGymnasium).
			Bind(pathId[Gymnasium]("gymnasium"))

		dataFetcher := pathId[MatchData]("matchdata")
		group.POST("/{matchdata}/assign", assignCourt).Bind(dataFetcher)
		group.POST("/{matchdata}/unassign", unassignCourt).Bind(dataFetcher)

		return e.Next()
	})
}

func deleteCourt(e *core.RequestEvent) error {
	court := e.Get("court").(*Court)

	if err := tops.DeleteCourt(e.App, court); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func deleteGymnasium(e *core.RequestEvent) error {
	gym := e.Get("gymnasium").(*Gymnasium)

	if err := tops.DeleteGymnasium(e.App, gym); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
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
	if err := e.BindBody(&data); err != nil {
		return nil, errors.New("the JSON body does not contain a 'court' fied")
	}

	courtId := data.CourtId
	court, err := store.FindProxy[Court](courtId)
	if err != nil {
		return nil, errors.New("court does not exist")
	}

	return court, nil
}
