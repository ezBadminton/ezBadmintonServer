package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindCompetitionHooks(app core.App) {
	cName := CName[Competition]()
	app.OnRecordDeleteRequest(cName).BindFunc(tops.DeleteCompetition)
}
