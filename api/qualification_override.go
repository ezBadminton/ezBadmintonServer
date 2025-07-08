package api

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
)

func BindQualificationOverrideHooks(app core.App) {
	url := "/qualificationoverride/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)
		group.Bind(pathId[Competition]("competition"))

		swapRoute := group.POST("/swap", swapQualificationOverridePositions)
		swapRoute.Bind(bodyId[Team]("teamA"))
		swapRoute.Bind(bodyId[Team]("teamB"))

		group.DELETE("", resetQualificationOverride)

		return e.Next()
	})
}

func swapQualificationOverridePositions(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)
	a := e.Get("teamA").(*Team)
	b := e.Get("teamB").(*Team)

	err := tops.QualificationOverrideSwap(e.App, competition, a, b)
	if err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func resetQualificationOverride(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	err := tops.QualificationOverrideReset(e.App, competition)
	if err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}
