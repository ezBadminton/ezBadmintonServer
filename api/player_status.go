package api

import (
	"errors"
	"net/http"
	"strconv"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const CompetitionsKey = "COMPETITIONS"

func BindPlayerStatusHooks(app core.App) {
	url := "/api/ezbadminton/playerstatus/{player}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("/{status}/preview", getStatusChangeList)
		group.POST("", setPlayerStatus)

		return e.Next()
	})
}

func getStatusChangeList(e *core.RequestEvent) error {
	player, err := findPathId[Player]("player", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	statusValue := e.Request.PathValue("status")
	statusI, err := strconv.Atoi(statusValue)
	if err != nil {
		return e.String(http.StatusBadRequest, "could not parse player status integer value")
	}
	status := PlayerStatus(statusI)
	if err := validatePlayerStatus(status); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	changes := tops.ListPlayerStatusChanges(player, status)

	return e.JSON(http.StatusOK, changes.ToMap())
}

func setPlayerStatus(e *core.RequestEvent) error {
	player, err := findPathId[Player]("player", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}
	status, err := readPlayerStatusFromBody(e)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}
	competitionIds := readCompetitionIdsFromBody(e)

	err = tops.SetPlayerStatus(e.App, player, status, competitionIds)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func readCompetitionIdsFromBody(e *core.RequestEvent) []string {
	data := struct {
		Competitions []string `json:"competitions"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return nil
	}
	return data.Competitions
}

func readPlayerStatusFromBody(e *core.RequestEvent) (PlayerStatus, error) {
	data := struct {
		Status int `json:"status"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return 0, errors.New("the JSON body does not contain the 'status' field")
	}
	status := PlayerStatus(data.Status)
	if err := validatePlayerStatus(status); err != nil {
		return 0, err
	}
	return status, nil
}

func validatePlayerStatus(status PlayerStatus) error {
	if status < NotAttending || status > Disqualified {
		return errors.New("invalid player status")
	}
	return nil
}
