package api

import (
	"errors"

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
	var competition *Competition
	params := e.Request.URL.Query()

	if params.Has("competition") {
		compId := params.Get("competition")
		compStore, _ := store.FindRecordStore[Competition]()
		competition, _ = compStore.FindProxy(compId)
		if competition == nil {
			return errors.New("the given competition ID was not found")
		}
	}

	team, _ := WrapRecord[Team](e.Record)
	store.ExpandRelationsDry(team)

	err := tops.Registrations.VerifyRegistration(team, competition)
	if err != nil {
		return err
	}

	return e.Next()
}

func listRegistrations(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.Registrations.List, e)
}
