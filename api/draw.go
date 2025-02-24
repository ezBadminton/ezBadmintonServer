package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindDrawHooks(app core.App) {
	url := "/draw/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)
		group.Bind(pathId[Competition]("competition"))

		group.POST("/make", makeDraw)
		group.POST("/swap", swapDrawPositions)
		group.POST("/redraw", redraw)
		group.POST("/seeds", setSeeds).
			Bind(bodyIdList[Team]("seeds"))
		group.DELETE("", deleteDraw)

		return e.Next()
	})
}

func makeDraw(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	if err := tops.MakeDraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func swapDrawPositions(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	data := struct {
		Swap []string `json:"swap"`
	}{}
	e.BindBody(&data)
	if len(data.Swap) == 0 {
		return e.String(http.StatusBadRequest, "the body did not contain JSON with a 'swap' string list field")
	}
	if len(data.Swap) != 2 || data.Swap[0] == data.Swap[1] {
		return e.String(http.StatusBadRequest, "the swap list does not have 2 unique IDs")
	}

	err := tops.DrawSwap(e.App, competition, data.Swap[0], data.Swap[1])
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func redraw(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	if err := tops.Redraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func deleteDraw(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)

	if err := tops.DeleteDraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func setSeeds(e *core.RequestEvent) error {
	competition := e.Get("competition").(*Competition)
	seeds := e.Get("seeds").([]*Team)

	if err := tops.SetSeeds(e.App, competition, seeds); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}
