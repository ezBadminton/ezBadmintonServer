package main

import (
	"fmt"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func RegisterHooks(app *pocketbase.PocketBase) {

	app.OnRecordBeforeUpdateRequest(names.Collections.Tournaments).Add(func(e *core.RecordUpdateEvent) error {
		return OnTournamentSettingsUpdate(e.Record.OriginalCopy(), e.Record, app.Dao())
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.PlayingLevels, names.Collections.AgeGroups).Add(func(e *core.RecordDeleteEvent) error {
		replacementCategoryId := e.HttpContext.QueryParamDefault("replacement", "")

		return HandleDeletedCategory(e.Record, replacementCategoryId, app.Dao())
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.Competitions).Add(func(e *core.RecordDeleteEvent) error {
		return HandleDeletedCompetition(e.Record, app.Dao())
	})

	app.OnRecordBeforeUpdateRequest(names.Collections.Teams).Add(func(e *core.RecordUpdateEvent) error {
		return HandleUpdatedTeam(e.Record, app.Dao())
	})

	app.OnRecordAfterCreateRequest(names.Collections.Teams).Add(func(e *core.RecordCreateEvent) error {
		competitionId := e.HttpContext.QueryParamDefault("competition", "")

		return HandleCreatedTeam(e.Record, competitionId, app.Dao())
	})

	app.OnRecordAfterUpdateRequest(names.Collections.Competitions).Add(func(e *core.RecordUpdateEvent) error {
		return HandleAfterCompetitionUpdated(e.Record, e.Record.OriginalCopy(), app.Dao())
	})

	app.OnRecordAfterUpdateRequest(names.Collections.MatchData).Add(func(e *core.RecordUpdateEvent) error {
		return HandleAfterUpdatedMatch(e.Record, e.Record.OriginalCopy(), app.Dao())
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.Gymnasiums).Add(func(e *core.RecordDeleteEvent) error {
		return HandleBeforeGymnasiumDelete(e.Record, app.Dao())
	})

	app.OnRecordBeforeCreateRequest(names.Collections.TournamentOrganizer).Add(func(e *core.RecordCreateEvent) error {
		return HandleBeforeTournamentOrganizerCreate(app.Dao())
	})

	app.OnModelAfterUpdate(names.Collections.Teams).Add(func(e *core.ModelEvent) error {
		return HandleAfterUpdatedTeam(e.Model, app.Dao())
	})

	// Register all relation update cascades
	app.OnBeforeServe().Add(func(_ *core.ServeEvent) error {
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.PlayingLevel, app)
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.Matches, app)
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.TieBreakers, app)
		// This relation cascade is handled by the HandleAfterUpdatedTeam hook
		//RegisterRelationUpdateCascade(competitionsName, registrationsName, app)

		RegisterRelationUpdateCascade(names.Collections.Teams, names.Fields.Teams.Players, app)

		RegisterRelationUpdateCascade(names.Collections.MatchData, names.Fields.MatchData.Court, app)
		RegisterRelationUpdateCascade(names.Collections.MatchData, names.Fields.MatchData.Sets, app)

		RegisterRelationUpdateCascade(names.Collections.Courts, names.Fields.Courts.Gymnasium, app)

		return nil
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Admin UI
		// (the isGoRun check is to enable it only during development)
		Automigrate: false,
	})

}

func RegisterRoutes(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.PUT(
			fmt.Sprintf("/api/ezbadminton/%s", names.Collections.MatchSets),
			func(c echo.Context) error { return PutMatchResult(c, app.Dao()) },
			apis.ActivityLogger(app),
			apis.RequireRecordAuth(),
		)

		e.Router.POST(
			fmt.Sprintf("/api/ezbadminton/%s", names.Collections.Competitions),
			func(c echo.Context) error { return PostCompetitionMatches(c, app.Dao()) },
			apis.ActivityLogger(app),
			apis.RequireRecordAuth(),
		)

		e.Router.GET(
			fmt.Sprintf("/api/ezbadminton/%s/exists", names.Collections.TournamentOrganizer),
			func(c echo.Context) error { return GetTournamentOrganizerExists(c, app.Dao()) },
			apis.ActivityLogger(app),
		)

		return nil
	})
}

func OnTournamentSettingsUpdate(old *models.Record, updated *models.Record, dao *daos.Dao) error {
	oldUseAgeGroups := old.GetBool(names.Fields.Tournaments.UseAgeGroups)
	updatedUseAgeGroups := updated.GetBool(names.Fields.Tournaments.UseAgeGroups)

	oldUsePlayingLevels := old.GetBool(names.Fields.Tournaments.UsePlayingLevels)
	updatedUsePlayingLevels := updated.GetBool(names.Fields.Tournaments.UsePlayingLevels)

	ageGroupsDisabled := oldUseAgeGroups && !updatedUseAgeGroups
	playingLevelsDisabled := oldUsePlayingLevels && !updatedUsePlayingLevels

	ageGroupsEnabled := !oldUseAgeGroups && updatedUseAgeGroups
	playingLevelsEnabled := !oldUsePlayingLevels && updatedUsePlayingLevels

	if err := HandleDisabledCategorization(ageGroupsDisabled, playingLevelsDisabled, dao); err != nil {
		return err
	}

	if err := HandleEnabledCategorization(ageGroupsEnabled, playingLevelsEnabled, dao); err != nil {
		return err
	}

	return nil
}
