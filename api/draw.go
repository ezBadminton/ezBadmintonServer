package api

import (
	"errors"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindDrawHooks(app core.App) {
	url := "/draw/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("/make", makeDraw)
		group.POST("/swap", swapDrawPositions)
		group.POST("/redraw", redraw)
		group.POST("/seeds", setSeeds)
		group.DELETE("", deleteDraw)

		return e.Next()
	})
}

func makeDraw(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.MakeDraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func swapDrawPositions(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	data := struct {
		Swap []string `json:"swap"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.String(http.StatusBadRequest, "the body did not contain JSON with a 'swap' string list field")
	}
	if len(data.Swap) != 2 || data.Swap[0] == data.Swap[1] {
		return e.String(http.StatusBadRequest, "the swap list does not have 2 unique IDs")
	}

	err = tops.DrawSwap(e.App, competition, data.Swap[0], data.Swap[1])
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func redraw(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.Redraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func deleteDraw(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.DeleteDraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func setSeeds(e *core.RequestEvent) error {
	competition, err := findPathId[Competition]("competition", e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}
	seeds, err := readSeedsFromBody(e)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.SetSeeds(e.App, competition, seeds); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func readSeedsFromBody(e *core.RequestEvent) ([]*Team, error) {
	data := struct {
		SeedIds []string `json:"seeds"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return nil, errors.New("the JSON body does not contain a 'seeds' field of team IDs")
	}

	seeds := make([]*Team, 0, len(data.SeedIds))
	for _, id := range data.SeedIds {
		team, err := store.FindProxy[Team](id)
		if err != nil {
			return nil, errors.New("the team ID does not exist")
		}
		seeds = append(seeds, team)
	}

	return seeds, nil
}
