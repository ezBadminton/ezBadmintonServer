package main

import (
	"net/http"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase/core"
)

// PutMatchResult processes PUT requests on the /api/ezbadminton/match_sets route.
// It creates the MatchSet records and assigns them to a match. Also deletes the old
// match sets of the match if they exist.
func PutMatchResult(e *core.RequestEvent, dao core.App) error {
	endTime := e.Request.URL.Query().Get("endTime")
	matchId := e.Request.URL.Query().Get("match")

	info, err := e.RequestInfo()
	if endTime == "" || matchId == "" || err != nil {
		return e.NoContent(http.StatusBadRequest)
	}
	resultsData, resultsExist := info.Body["results"]
	if !resultsExist {
		return e.NoContent(http.StatusBadRequest)
	}

	var resultsDataArray []interface{}

	switch val := resultsData.(type) {
	case []interface{}:
		resultsDataArray = val
	default:
		return e.NoContent(http.StatusBadRequest)
	}

	var resultArray []int = make([]int, 0, len(resultsDataArray))

	for _, result := range resultsDataArray {
		switch val := result.(type) {
		case float64:
			resultArray = append(resultArray, int(val))
		default:
			return e.NoContent(http.StatusBadRequest)
		}
	}

	numScores := len(resultArray)

	if numScores == 0 || numScores%2 != 0 {
		return e.NoContent(http.StatusBadRequest)
	}

	transactionError := dao.RunInTransaction(func(txDao core.App) error {
		matchSetCollection, err := txDao.FindCollectionByNameOrId(names.Collections.MatchSets)
		if err != nil {
			return err
		}
		match, err := txDao.FindRecordById(names.Collections.MatchData, matchId)
		if err != nil {
			return err
		}

		txDao.ExpandRecord(match, []string{names.Fields.MatchData.Sets}, nil)
		oldSets := match.ExpandedAll(names.Fields.MatchData.Sets)

		newSetIds := make([]string, 0, 2)

		for i := 0; i < numScores; i += 2 {
			newSet := core.NewRecord(matchSetCollection)

			newSet.Set(names.Fields.MatchSets.Team1Points, resultArray[i])
			newSet.Set(names.Fields.MatchSets.Team2Points, resultArray[i+1])

			if err := txDao.Save(newSet); err != nil {
				return err
			}

			newSetIds = append(newSetIds, newSet.Id)
		}

		match.Set(names.Fields.MatchData.EndTime, endTime)
		match.Set(names.Fields.MatchData.Sets, newSetIds)
		if err := txDao.Save(match); err != nil {
			return err
		}

		if err := ProcessAsModels(oldSets, txDao.Delete); err != nil {
			return err
		}

		return nil
	})

	if transactionError != nil {
		return e.NoContent(http.StatusInternalServerError)
	}

	return e.NoContent(http.StatusOK)
}

// HandleAfterUpdatedMatch deletes the match's sets if they have been removed from the match
func HandleAfterUpdatedMatch(updatedMatch *core.Record, oldMatch *core.Record, dao core.App) error {
	updatedSetIds := updatedMatch.GetStringSlice(names.Fields.MatchData.Sets)
	oldSetIds := oldMatch.GetStringSlice(names.Fields.MatchData.Sets)

	numUpdatedSets := len(updatedSetIds)
	numOldSets := len(oldSetIds)

	if numUpdatedSets > 0 || numOldSets == 0 {
		return nil
	}

	if err := DeleteModelsById(names.Collections.MatchSets, oldSetIds, dao); err != nil {
		return err
	}

	return nil
}
