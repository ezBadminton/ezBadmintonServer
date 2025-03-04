package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindTieBreakerHooks(app core.App) {
	url := "/tiebreakers"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("/{competition}", addTieBreaker).Bind(
			pathId[Competition]("competition"),
			bodyIdList[Team]("teams"),
		)
		group.PATCH("/{tiebreaker}", updateTieBreaker).Bind(
			pathId[TieBreaker]("tiebreaker"),
			bodyIdList[Team]("teams"),
		)
		group.DELETE("/{tiebreaker}", deleteTieBreaker).
			Bind(pathId[TieBreaker]("tiebreaker"))

		return e.Next()
	})
}

func addTieBreaker(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)
	teams := e.Get("teams").([]*Team)

	if err := tops.AddTieBreaker(e.App, competition, teams); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func updateTieBreaker(e *core.RequestEvent) error {
	tieBreaker := e.Get("tiebreaker").(*TieBreaker)
	teams := e.Get("teams").([]*Team)

	if err := tops.UpdateTieBreaker(e.App, tieBreaker, teams); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}

func deleteTieBreaker(e *core.RequestEvent) error {
	tieBreaker := e.Get("tiebreaker").(*TieBreaker)

	if err := tops.DeleteTieBreaker(e.App, tieBreaker); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	return e.NoContent(http.StatusOK)
}
