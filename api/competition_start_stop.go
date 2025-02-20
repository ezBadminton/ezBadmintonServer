package api

import (
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindStartStopHooks(app core.App) {
	url := "/api/ezbadminton/startstop/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.POST("", handleStartStop)

		return e.Next()
	})
}

func handleStartStop(e *core.RequestEvent) error {
	competition, err := findCompetition(e)
	if competition == nil {
		return err
	}
	tournament := tops.Tournaments.FindTournament(competition.Id)
	if tournament == nil {
		return e.String(http.StatusBadRequest, "competition has no draw")
	}

	body := struct {
		Started bool `json:"started"`
	}{}
	if err := e.BindBody(&body); err != nil {
		return e.String(http.StatusBadRequest, "the body does not contain JSON with one 'started' bool field")
	}

	if body.Started {
		return startCompetition(e, tournament)
	} else {
		return stopCompetition(e, tournament)
	}
}

func startCompetition(e *core.RequestEvent, tournament *tops.CompetitionTournament) error {
	comp := tournament.Competition
	started, err := tops.Tournaments.HasStarted(comp.Id)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}
	if started {
		return e.String(http.StatusBadRequest, "competition already running")
	}

	matches := tournament.MatchList().Matches
	matchData := make([]*MatchData, len(matches))
	for i := range matches {
		data, err := NewProxy[MatchData](e.App)
		if err != nil {
			return e.NoContent(http.StatusInternalServerError)
		}
		matchData[i] = data
	}

	comp, _ = WrapRecord[Competition](tournament.Competition.Clone())
	comp.SetMatches(matchData)
	if err := e.App.Save(comp); err != nil {
		return e.NoContent(http.StatusInternalServerError)
	}

	if err := tops.Tournaments.SetStarted(comp.Id, true); err != nil {
		return e.NoContent(http.StatusInternalServerError)
	}

	return nil
}

func stopCompetition(e *core.RequestEvent, tournament *tops.CompetitionTournament) error {
	comp := tournament.Competition
	started, err := tops.Tournaments.HasStarted(comp.Id)
	if err != nil {
		return e.JSON(http.StatusBadRequest, err)
	}
	if !started {
		return e.JSON(http.StatusBadRequest, "competition not running")
	}

	comp, _ = WrapRecord[Competition](tournament.Competition.Clone())
	comp.SetMatches(nil)
	if err := e.App.Save(comp); err != nil {
		return e.JSON(http.StatusInternalServerError, nil)
	}

	if err := tops.Tournaments.SetStarted(comp.Id, false); err != nil {
		return e.JSON(http.StatusInternalServerError, nil)
	}

	return nil
}
