package api

import (
	"errors"
	"net/http"
	"strconv"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindPlayerStatusHooks(app core.App) {
	url := "/playerstatus/{player}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)
		group.Bind(pathId[Player]("player"))

		group.GET("/{status}/preview", getStatusChangeList)
		group.POST("", setPlayerStatus).
			Bind(bodyIdList[Competition]("competitions"))

		return e.Next()
	})
}

func getStatusChangeList(e *core.RequestEvent) error {
	player := e.Get("player").(*Player)

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
	player := e.Get("player").(*Player)
	status, err := readPlayerStatusFromBody(e)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}
	competitions := e.Get("competitions").([]*Competition)

	err = tops.SetPlayerStatus(e.App, player, status, competitions)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func readPlayerStatusFromBody(e *core.RequestEvent) (PlayerStatus, error) {
	data := struct {
		Status int `json:"status"`
	}{-999}
	e.BindBody(&data)
	if data.Status == -999 {
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
