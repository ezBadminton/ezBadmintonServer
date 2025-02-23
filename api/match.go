package api

import (
	"errors"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindMatchHooks(app core.App) {
	url := "/matches/{matchdata}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("/start", startMarch)
		group.POST("/cancel", cancelMatch)
		group.POST("/score", setMatchScore)
		group.POST("/reset", resetMatch)

		return e.Next()
	})
}

func startMarch(e *core.RequestEvent) error {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.StartMatch(e.App, matchData); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func cancelMatch(e *core.RequestEvent) error {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.CancelMatch(e.App, matchData); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func setMatchScore(e *core.RequestEvent) error {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	points, err := readPointsFromBody(e)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.SetMatchScore(e.App, matchData, points); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func resetMatch(e *core.RequestEvent) error {
	matchData, err := findPathId[MatchData]("matchdata", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.ResetMatch(e.App, matchData); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func readPointsFromBody(e *core.RequestEvent) ([][]int, error) {
	data := struct {
		Team1Points []int `json:"team1points"`
		Team2Points []int `json:"team2points"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return nil, errors.New("the JSON body does not contain the 'team1points' and 'team2points' fields")
	}
	points := [][]int{data.Team1Points, data.Team2Points}
	return points, nil
}
