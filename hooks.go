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
		if err := OnTournamentSettingsUpdate(e.Record.OriginalCopy(), e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.PlayingLevels, names.Collections.AgeGroups).Add(func(e *core.RecordDeleteEvent) error {
		replacementCategoryId := e.HttpContext.QueryParamDefault("replacement", "")

		if err := HandleDeletedCategory(e.Record, replacementCategoryId, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.Competitions).Add(func(e *core.RecordDeleteEvent) error {
		if err := HandleDeletedCompetition(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeUpdateRequest(names.Collections.Teams).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleUpdatedTeam(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterCreateRequest(names.Collections.Teams).Add(func(e *core.RecordCreateEvent) error {
		competitionId := e.HttpContext.QueryParamDefault("competition", "")

		if err := HandleCreatedTeam(e.Record, competitionId, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterUpdateRequest(names.Collections.Competitions).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleAfterCompetitionUpdated(e.Record, e.Record.OriginalCopy(), app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterUpdateRequest(names.Collections.MatchData).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleAfterUpdatedMatch(e.Record, e.Record.OriginalCopy(), app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(names.Collections.Gymnasiums).Add(func(e *core.RecordDeleteEvent) error {
		if err := HandleBeforeGymnasiumDelete(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnModelAfterUpdate(names.Collections.Teams).Add(func(e *core.ModelEvent) error {
		if err := HandleAfterUpdatedTeam(e.Model, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	// Register all relation update cascades
	app.OnAfterBootstrap().Add(func(_ *core.BootstrapEvent) error {
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
