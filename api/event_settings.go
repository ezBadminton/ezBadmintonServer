package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindEventSettingsHooks(app core.App) {
	cName := CName[TournamentEvent]()
	app.OnRecordUpdateRequest(cName).BindFunc(tops.ChangeEventSettings)
}
