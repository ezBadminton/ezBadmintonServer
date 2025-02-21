package api

import (
	"net/http"
	"strconv"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const CompetitionsKey = "COMPETITIONS"

func BindPlayerStatusHooks(app core.App) {
	cName := CName[Player]()
	url := "/api/ezbadminton/statuschangelist/{player}/{status}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("", getStatusChangeList)

		return e.Next()
	})

	app.OnRecordUpdateRequest(cName).BindFunc(onPlayerWithCompetitionIds)
	app.OnRecordUpdate(cName).BindFunc(onPlayerStatusChange)
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
	if status < NotAttending || status > Disqualified {
		return e.String(http.StatusBadRequest, "invalid player status")
	}

	changes := tops.ListPlayerStatusChanges(player, status)

	return e.JSON(http.StatusOK, changes.ToMap())
}

func onPlayerWithCompetitionIds(e *core.RecordRequestEvent) error {
	competitionIds := readCompetitionIds(e.RequestEvent)
	if len(competitionIds) > 0 {
		e.Record.SetRaw(CompetitionsKey, competitionIds)
	}
	return e.Next()
}

func onPlayerStatusChange(e *core.RecordEvent) error {
	oldPlayer, player, err := oldNew[Player](e, false)
	if err != nil {
		return err
	}

	oldStatus, status := oldPlayer.Status(), player.Status()
	if oldStatus == status {
		return e.Next()
	}

	var withdraw bool
	if oldStatus == Attending && status != Attending {
		withdraw = true
	} else if oldStatus != Attending && status == Attending {
		withdraw = false
	} else {
		return e.Next()
	}

	customData := e.Record.CustomData()
	competitionIds, ok := customData[CompetitionsKey].([]string)
	if !ok {
		return e.Next()
	}

	app := e.App
	err = e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		changes, err := tops.WithdrawOrReenterPlayer(e.App, player, competitionIds, withdraw)
		if err != nil {
			return err
		}

		e.Record.SetRaw(tops.StatusChangeKey, changes)

		return e.Next()
	})
	e.App = app

	return err
}

func readCompetitionIds(e *core.RequestEvent) []string {
	data := struct {
		Competitions []string `json:"competitions"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return nil
	}
	return data.Competitions
}
