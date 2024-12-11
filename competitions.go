package main

import (
	"errors"
	"net/http"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase/core"
)

// HandleAfterCompetitionUpdated deletes the matches and sets of a competition when the competition has been cancelled
func HandleAfterCompetitionUpdated(updatedCompetition *core.Record, oldCompetition *core.Record, dao core.App) error {
	matches := updatedCompetition.GetStringSlice(names.Fields.Competitions.Matches)
	oldMatchIds := oldCompetition.GetStringSlice(names.Fields.Competitions.Matches)

	numMatches := len(matches)
	oldNumMatches := len(oldMatchIds)

	if numMatches > 0 || oldNumMatches == 0 {
		return nil
	}

	if err := dao.ExpandRecord(oldCompetition, []string{names.Fields.Competitions.Matches}, nil); len(err) != 0 {
		return err[names.Fields.Competitions.Matches]
	}

	oldMatches := oldCompetition.ExpandedAll(names.Fields.Competitions.Matches)

	if err := dao.ExpandRecords(oldMatches, []string{names.Fields.MatchData.Sets}, nil); len(err) != 0 {
		return err[names.Fields.MatchData.Sets]
	}

	oldSetIds := make([]string, 0, 2*len(oldMatches))
	for _, match := range oldMatches {
		for _, set := range match.ExpandedAll(names.Fields.MatchData.Sets) {
			oldSetIds = append(oldSetIds, set.Id)
		}
	}

	if err := DeleteModelsById(names.Collections.MatchData, oldMatchIds, dao); err != nil {
		return err
	}

	if err := DeleteModelsById(names.Collections.MatchSets, oldSetIds, dao); err != nil {
		return err
	}

	return nil
}

// PostCompetitionMatches handles POST requests to the /api/ezbadminton/competitions route.
// It creates the given amount of MatchData records and assigns them to the competition.
// This starts the competition.
func PostCompetitionMatches(e *core.RequestEvent, dao core.App) error {
	info, err := e.RequestInfo()
	if err != nil {
		return e.NoContent(http.StatusBadRequest)
	}

	body := info.Body

	competitionIdData, competitionIdExists := body["competition"]
	numMatchesData, numMatchesExists := body["numMatches"]

	if !competitionIdExists || !numMatchesExists {
		return e.NoContent(http.StatusBadRequest)
	}

	var competitionId string
	var numMatches int

	switch val := competitionIdData.(type) {
	case string:
		competitionId = val
	default:
		return e.NoContent(http.StatusBadRequest)
	}

	switch val := numMatchesData.(type) {
	case float64:
		numMatches = int(val)
	default:
		return e.NoContent(http.StatusBadRequest)
	}

	transactionError := dao.RunInTransaction(func(txDao core.App) error {
		matchDataCollection, err := txDao.FindCollectionByNameOrId(names.Collections.MatchData)
		if err != nil {
			return err
		}
		competition, err := txDao.FindRecordById(names.Collections.Competitions, competitionId)
		if err != nil {
			return err
		}

		isCompetitionRunning := len(competition.GetStringSlice(names.Fields.Competitions.Matches)) != 0

		if isCompetitionRunning {
			return errors.New("cannot start an already running competition")
		}

		newMatchIds := make([]string, 0, numMatches)

		for i := 0; i < numMatches; i += 1 {
			newMatch := core.NewRecord(matchDataCollection)

			if err := txDao.Save(newMatch); err != nil {
				return err
			}

			newMatchIds = append(newMatchIds, newMatch.Id)
		}

		competition.Set(names.Fields.Competitions.Matches, newMatchIds)

		if err := txDao.Save(competition); err != nil {
			return err
		}

		return nil
	})

	if transactionError != nil {
		return e.NoContent(http.StatusInternalServerError)
	}

	return nil
}
