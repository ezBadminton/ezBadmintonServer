package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindPlayerStatusHooks(app core.App) {
	cName := CName[Player]()

	app.OnRecordUpdateRequest(cName).BindFunc(onPlayerWithCompetitionIds)
	app.OnRecordUpdate(cName).BindFunc(onPlayerStatusChange)
}

func onPlayerWithCompetitionIds(e *core.RecordRequestEvent) error {
	competitionIds := readCompetitionIds(e.RequestEvent)
	if len(competitionIds) > 0 {
		e.Record.SetRaw("COMPETITIONS", competitionIds)
	}
	return e.Next()
}

func onPlayerStatusChange(e *core.RecordEvent) error {
	oldPlayer, player, err := oldNew[Player](e)
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
	competitionIds, ok := customData["COMPETITIONS"].([]string)
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

		e.Record.SetRaw("STATUS_CHANGES", changes)

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
