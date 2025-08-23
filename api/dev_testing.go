package api

import (
	"net/http"

	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindDevTestingHooks(app core.App) {
	if BuildVersion != "dev" {
		return
	}

	url := "/dev"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("/createplayers", massCreatePlayers)

		return e.Next()
	})
}

func massCreatePlayers(e *core.RequestEvent) error {
	data := struct {
		Amount int `json:"amount"`
	}{-1}
	e.BindBody(&data)

	amountOfPlayers := data.Amount
	if amountOfPlayers < 0 {
		return e.BadRequestError("body does not contain 'amount' field with positive integer", nil)
	}

	if err := tops.CreateTestPlayers(e.App, amountOfPlayers); err != nil {
		return e.InternalServerError("could not create test players", err)
	}

	return e.NoContent(http.StatusCreated)
}
