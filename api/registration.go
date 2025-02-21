package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindRegistrationHooks(app core.App) {
	url := "/api/collections/registrations"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("/records", listRegistrations)

		return e.Next()
	})

	cName := CName[Team]()

	app.OnRecordCreateRequest(cName).BindFunc(CheckRegistration)
	app.OnRecordUpdateRequest(cName).BindFunc(CheckRegistration)
}

func CheckRegistration(e *core.RecordRequestEvent) error {
	competition, _ := findPathId[Competition]("competition", e.Request)

	team, _ := WrapRecord[Team](e.Record)
	store.ExpandRelationsDry(team)

	err := tops.VerifyRegistration(team, competition)
	if err != nil {
		return err
	}

	return e.Next()
}

func listRegistrations(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListRegistrations(), e)
}
