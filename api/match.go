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
		group.Bind(pathId[MatchData]("matchdata"))

		group.POST("/start", startMarch)
		group.POST("/cancel", cancelMatch)
		group.POST("/end", endMatch)
		group.POST("/score", setMatchScore)
		group.POST("/reset", resetMatch)

		return e.Next()
	})
}

func startMarch(e *core.RequestEvent) error {
	matchData := e.Get("matchdata").(*MatchData)

	if err := tops.StartMatch(e.App, matchData); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func cancelMatch(e *core.RequestEvent) error {
	matchData := e.Get("matchdata").(*MatchData)

	if err := tops.CancelMatch(e.App, matchData); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func endMatch(e *core.RequestEvent) error {
	matchData := e.Get("matchdata").(*MatchData)

	if err := tops.EndMatch(e.App, matchData); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func setMatchScore(e *core.RequestEvent) error {
	matchData := e.Get("matchdata").(*MatchData)

	points, err := readPointsFromBody(e)
	if err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	if err := tops.SetMatchScore(e.App, matchData, points); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func resetMatch(e *core.RequestEvent) error {
	matchData := e.Get("matchdata").(*MatchData)

	if err := tops.ResetMatch(e.App, matchData); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func readPointsFromBody(e *core.RequestEvent) ([][]int, error) {
	data := struct {
		Team1Points []int `json:"team1points"`
		Team2Points []int `json:"team2points"`
	}{}
	e.BindBody(&data)
	if len(data.Team1Points) == 0 || len(data.Team2Points) == 0 {
		return nil, errors.New("the JSON body does not contain the 'team1points' and 'team2points' fields")
	}
	points := [][]int{data.Team1Points, data.Team2Points}
	return points, nil
}
