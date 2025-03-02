package api

import (
	"github.com/pocketbase/pocketbase/core"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
)

func BindTournamentModeSettingsHooks(app core.App) {
	cName := CName[TournamentModeSettings]()
	app.OnRecordCreateRequest(cName).BindFunc(handleSettingsCreate)
}

func handleSettingsCreate(e *core.RecordRequestEvent) error {
	idListGetter := bodyIdList[Competition]("competitions").Func
	if err := idListGetter(e.RequestEvent); err != nil {
		return err
	}

	return tops.SetTournamentModeSettings(e)
}
