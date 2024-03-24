package main

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

func RegisterHooks(app *pocketbase.PocketBase) {

	app.OnRecordBeforeUpdateRequest(tournamentsName).Add(func(e *core.RecordUpdateEvent) error {
		if err := OnTournamentSettingsUpdate(e.Record.OriginalCopy(), e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(playingLevelsName, ageGroupsName).Add(func(e *core.RecordDeleteEvent) error {
		replacementCategoryId := e.HttpContext.QueryParamDefault("replacement", "")

		if err := HandleDeletedCategory(e.Record, replacementCategoryId, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(competitionsName).Add(func(e *core.RecordDeleteEvent) error {
		if err := HandleDeletedCompetition(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeUpdateRequest(teamsName).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleUpdatedTeam(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterCreateRequest(teamsName).Add(func(e *core.RecordCreateEvent) error {
		competitionId := e.HttpContext.QueryParamDefault("competition", "")

		if err := HandleCreatedTeam(e.Record, competitionId, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterUpdateRequest(competitionsName).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleAfterCompetitionUpdated(e.Record, e.Record.OriginalCopy(), app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordAfterUpdateRequest(matchDataName).Add(func(e *core.RecordUpdateEvent) error {
		if err := HandleAfterUpdatedMatch(e.Record, e.Record.OriginalCopy(), app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnRecordBeforeDeleteRequest(gymnasiumsName).Add(func(e *core.RecordDeleteEvent) error {
		if err := HandleBeforeGymnasiumDelete(e.Record, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	app.OnModelAfterUpdate(teamsName).Add(func(e *core.ModelEvent) error {
		if err := HandleAfterUpdatedTeam(e.Model, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	// Register all relation update cascades
	app.OnAfterBootstrap().Add(func(_ *core.BootstrapEvent) error {
		RegisterRelationUpdateCascade(competitionsName, playingLevelName, app)
		RegisterRelationUpdateCascade(competitionsName, competitionMatchesName, app)
		RegisterRelationUpdateCascade(competitionsName, competitionTieBreakersName, app)
		// This relation cascade is handled by the HandleAfterUpdatedTeam hook
		//RegisterRelationUpdateCascade(competitionsName, registrationsName, app)

		RegisterRelationUpdateCascade(teamsName, teamPlayersName, app)

		RegisterRelationUpdateCascade(matchDataName, matchDataCourtName, app)
		RegisterRelationUpdateCascade(matchDataName, matchDataSetsName, app)

		RegisterRelationUpdateCascade(courtsName, courtGymnasiumName, app)

		return nil
	})

}

func RegisterRoutes(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.PUT(
			fmt.Sprintf("/api/ezbadminton/%s", matchSetsName),
			func(c echo.Context) error { return PutMatchResult(c, app.Dao()) },
			apis.ActivityLogger(app),
			apis.RequireRecordAuth(),
		)

		e.Router.POST(
			fmt.Sprintf("/api/ezbadminton/%s", competitionsName),
			func(c echo.Context) error { return PostCompetitionMatches(c, app.Dao()) },
			apis.ActivityLogger(app),
			apis.RequireRecordAuth(),
		)

		return nil
	})
}

func OnTournamentSettingsUpdate(old *models.Record, updated *models.Record, dao *daos.Dao) error {
	oldUseAgeGroups := old.GetBool(useAgeGroupsName)
	updatedUseAgeGroups := updated.GetBool(useAgeGroupsName)

	oldUsePlayingLevels := old.GetBool(usePlayingLevelsName)
	updatedUsePlayingLevels := updated.GetBool(usePlayingLevelsName)

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
