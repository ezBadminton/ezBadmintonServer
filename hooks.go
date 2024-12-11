package main

import (
	"fmt"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func RegisterHooks(app *pocketbase.PocketBase) {

	app.OnRecordUpdateRequest(names.Collections.Tournaments).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := OnTournamentSettingsUpdate(e.Record.Original(), e.Record, app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordDeleteRequest(names.Collections.PlayingLevels, names.Collections.AgeGroups).BindFunc(func(e *core.RecordRequestEvent) error {
		replacementCategoryId := e.Request.URL.Query().Get("replacement")

		if err := HandleDeletedCategory(e.Record, replacementCategoryId, app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordDeleteRequest(names.Collections.Competitions).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := HandleDeletedCompetition(e.Record, app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdateRequest(names.Collections.Teams).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := HandleUpdatedTeam(e.Record, app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordCreateRequest(names.Collections.Teams).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		competitionId := e.Request.URL.Query().Get("competition")

		return HandleCreatedTeam(e.Record, competitionId, app)
	})

	app.OnRecordUpdateRequest(names.Collections.Competitions).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := e.Next(); err != nil {
			return nil
		}
		return HandleAfterCompetitionUpdated(e.Record, e.Record.Original(), app)
	})

	app.OnRecordUpdateRequest(names.Collections.MatchData).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return HandleAfterUpdatedMatch(e.Record, e.Record.Original(), app)
	})

	app.OnRecordDeleteRequest(names.Collections.Gymnasiums).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := HandleBeforeGymnasiumDelete(e.Record, app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordCreateRequest(names.Collections.TournamentOrganizer).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := HandleBeforeTournamentOrganizerCreate(app); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdate(names.Collections.Teams).BindFunc(func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return HandleAfterUpdatedTeam(e.Record, app)
	})

	// Register all relation update cascades
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.PlayingLevel, app)
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.Matches, app)
		RegisterRelationUpdateCascade(names.Collections.Competitions, names.Fields.Competitions.TieBreakers, app)
		// This relation cascade is handled by the HandleAfterUpdatedTeam hook
		//RegisterRelationUpdateCascade(competitionsName, registrationsName, app)

		RegisterRelationUpdateCascade(names.Collections.Teams, names.Fields.Teams.Players, app)

		RegisterRelationUpdateCascade(names.Collections.MatchData, names.Fields.MatchData.Court, app)
		RegisterRelationUpdateCascade(names.Collections.MatchData, names.Fields.MatchData.Sets, app)

		RegisterRelationUpdateCascade(names.Collections.Courts, names.Fields.Courts.Gymnasium, app)

		return e.Next()
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Admin UI
		// (the isGoRun check is to enable it only during development)
		Automigrate: false,
	})

}

func RegisterRoutes(app *pocketbase.PocketBase) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.PUT(
			fmt.Sprintf("/api/ezbadminton/%s", names.Collections.MatchSets),
			func(e *core.RequestEvent) error {
				return PutMatchResult(e, app)
			},
		).Bind(apis.RequireAuth())

		e.Router.POST(
			fmt.Sprintf("/api/ezbadminton/%s", names.Collections.Competitions),
			func(e *core.RequestEvent) error { return PostCompetitionMatches(e, app) },
		).Bind(apis.RequireAuth())

		e.Router.GET(
			fmt.Sprintf("/api/ezbadminton/%s/exists", names.Collections.TournamentOrganizer),
			func(e *core.RequestEvent) error { return GetTournamentOrganizerExists(e, app) },
		)

		return e.Next()
	})
}

func OnTournamentSettingsUpdate(old *core.Record, updated *core.Record, dao core.App) error {
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
