package api

import (
	"errors"
	"net/http"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindDrawHooks(app core.App) {
	url := "/api/ezbadminton/draw/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.GET("", getDraw)
		group.POST("/swap", swapDrawPositions)

		return e.Next()
	})

	cName := CName[Competition]()
	app.OnRecordUpdate(cName).BindFunc(handleDrawChange)
	app.OnRecordUpdate(cName).BindFunc(checkSeeds)
}

// Responds with a list of Team IDs which are the draw.
// If no draw exists, an attempt is made to create one.
func getDraw(e *core.RequestEvent) error {
	competition, err := findCompetition(e.Request)
	if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	if err := tops.MakeDraw(e.App, competition); err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

func swapDrawPositions(e *core.RequestEvent) error {
	competition, err := findCompetition(e.Request)
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
	if errors.Is(err, tops.ErrUnexpected) {
		return e.NoContent(http.StatusInternalServerError)
	} else if err != nil {
		return e.String(http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusOK)
}

// Checks for a change in the draw of a competition
// and if one is found, (re-)creates a new tournament
// with the new draw.
func handleDrawChange(e *core.RecordEvent) error {
	oldCompetition, competition, err := oldNew[Competition](e)
	if err != nil {
		return err
	}

	oldDraw := oldCompetition.Draw()
	draw := competition.Draw()

	didChange := !slices.EqualFunc(
		oldDraw, draw,
		func(a, b *Team) bool { return a.Id == b.Id },
	)

	if !didChange {
		return e.Next()
	}

	tournament, err := tops.CreateTournament(competition)
	if err != nil {
		return err
	}

	// The update handler of the tournament operations (tops/tournaments.go)
	// reads this custom record data and stores it in the TournamentStore
	// It is not done here to avoid persisting a tournament when
	// the transaction of the draw change is unsuccessful.
	e.Record.SetRaw("DRAW_CHANGE", tournament)

	return e.Next()
}

func checkSeeds(e *core.RecordEvent) error {
	competition, _ := WrapRecord[Competition](e.Record)
	if err := store.ExpandRelationsDry(competition); err != nil {
		return err
	}

	seeds := competition.Seeds()
	registrations := competition.Registrations()

	if !containsAll(registrations, seeds) {
		return errors.New("can not set a seed for an unregistered team")
	}

	return e.Next()
}
