package api

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
)

func BindStartingFeeHooks(app core.App) {
	url := "/startingfees"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("", setStartingFees).
			Bind(bodyIdList[Competition]("competitions"))
		group.POST("/{player}", payStartingFee).
			Bind(pathId[Player]("player"))
		group.POST("/massdiscount", updateOrCreateMassDiscount)

		return e.Next()
	})
}

func setStartingFees(e *core.RequestEvent) error {
	competitions := e.Get("competitions").([]*Competition)

	data := struct {
		StartingFee int `json:"startingFee"`
	}{-1}
	e.BindBody(&data)
	startingFee := data.StartingFee
	if data.StartingFee < 0 {
		return e.BadRequestError("did not receive a positive integer 'startingFee' body field", nil)
	}

	if err := tops.SetStartingFees(competitions, startingFee); err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	return e.NoContent(http.StatusOK)
}

func payStartingFee(e *core.RequestEvent) error {
	player := e.Get("player").(*Player)
	data := struct {
		Amount          int `json:"amount"`
		DiscountPercent int `json:"discountPercent"`
	}{-1, 0}
	e.BindBody(&data)
	amount := data.Amount
	discountPercent := data.DiscountPercent
	if amount <= 0 {
		return e.BadRequestError("did not receive a non-zero positive integer 'amount' body field", nil)
	}

	if err := tops.PayStartingFee(player, amount, discountPercent); err != nil {
		if errors.Is(err, tops.ErrInvalidDiscount) || errors.Is(err, tops.ErrInvalidPaymentAmount) {
			return e.BadRequestError(err.Error(), nil)
		}

		return e.InternalServerError("something went wrong", err)
	}

	return e.NoContent(http.StatusOK)
}

func updateOrCreateMassDiscount(e *core.RequestEvent) error {
	data := struct {
		MinRegistrations int `json:"minRegistrations"`
		Amount           int `json:"amount"`
	}{-1, -1}
	e.BindBody(&data)
	minRegistrations := data.MinRegistrations
	amount := data.Amount
	if minRegistrations < 1 || amount < 1 {
		return e.BadRequestError("did not receive integers greater than 0 in the 'minRegistrations' and 'amount' body fields", nil)
	}

	if err := tops.UpdateOrCreateMassDiscount(minRegistrations, amount); err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	return e.NoContent(http.StatusOK)
}
