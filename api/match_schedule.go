package api

import (
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindMatchScheduleHooks(app core.App) {
	url := "/api/collections"
	scheduleUrl := "/schedule/records"
	roundUrl := "/scheduled_rounds/records"
	matchUrl := "/scheduled_matches/records"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET(scheduleUrl, listSchedule)
		group.GET(roundUrl, listScheduledRounds)
		group.GET(matchUrl, listScheduledMatches)

		return e.Next()
	})
}

func listSchedule(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListSchedule(), e)
}
func listScheduledRounds(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListScheduledRounds(), e)
}
func listScheduledMatches(e *core.RequestEvent) error {
	return tops.TopsRecordListResponse(tops.ListScheduledMatches(), e)
}
