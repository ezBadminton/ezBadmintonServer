package tops

import (
	"errors"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/gotournament/badminton"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func InitMatches() error {
	return nil
}

func startMatch(app core.App, matchData *MatchData) error {
	matchStatus := Schedule.scheduleStatus(matchData)
	if matchStatus != Ready {
		return errors.New("the match is not in the ready state and can not be started")
	}

	startTime := types.NowDateTime()

	matchData = Clone(matchData)
	matchData.SetStartTime(startTime)
	if err := app.Save(matchData); err != nil {
		return err
	}

	match := Tournaments.matches[matchData.Id]
	match.StartTime = startTime.Time()

	tournament := Tournaments.byMatch[matchData.Id]
	tournament.UpdateEditableMatches()

	Schedule.setMatchScheduleStatus(matchData, InProgress, nil)

	return nil
}

func cancelMatch(app core.App, matchData *MatchData) error {
	matchStatus := Schedule.scheduleStatus(matchData)
	if matchStatus != InProgress {
		return errors.New("the match is not in progress and can not be canceled")
	}

	matchData = Clone(matchData)
	matchData.SetStartTime(types.DateTime{})
	if err := app.Save(matchData); err != nil {
		return err
	}

	match := Tournaments.matches[matchData.Id]
	match.StartTime = time.Time{}

	tournament := Tournaments.byMatch[matchData.Id]
	tournament.UpdateEditableMatches()

	Schedule.setMatchScheduleStatus(matchData, Ready, nil)

	return nil
}

func setMatchScore(app core.App, matchData *MatchData, points [][]int) error {
	matchStatus := Schedule.scheduleStatus(matchData)
	if matchStatus == Done {
		if !isEditable(matchData) {
			return errors.New("the match is not editable")
		}
	} else if matchStatus != InProgress {
		return errors.New("the match is not in progress and can not have its score set")
	}
	if len(points) != 2 {
		return errors.New("invalid score")
	}

	matchData = Clone(matchData)

	tournament := Tournaments.byMatch[matchData.Id]
	score, err := badminton.NewScore(points[0], points[1], tournament.ScoreSettings)
	if err != nil {
		return errors.New("invalid score")
	}
	scoreData := make([]*MatchSet, 0, 3)
	endTime := types.NowDateTime()

	err = app.RunInTransaction(func(txApp core.App) error {
		for i := range len(points[0]) {
			set, err := NewProxy[MatchSet](txApp)
			if err != nil {
				return err
			}
			set.SetTeam1Points(points[0][i])
			set.SetTeam2Points(points[1][i])
			if err := txApp.Save(set); err != nil {
				return err
			}
			scoreData = append(scoreData, set)
		}

		if err := deleteScoreData(txApp, matchData); err != nil {
			return err
		}

		if matchStatus == InProgress {
			matchData.SetEndTime(endTime)
		}

		matchData.SetSets(scoreData)
		if err := txApp.Save(matchData); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	match := Tournaments.matches[matchData.Id]
	match.Score = score
	if matchStatus == InProgress {
		match.EndTime = endTime.Time()
		Schedule.setMatchScheduleStatus(matchData, Done, matchData.Court())
	}

	tournament.Update(nil)

	if matchStatus == InProgress {
		Schedule.updateTournamentScheduleStatus(tournament)
	}

	return nil
}

func resetMatch(app core.App, matchData *MatchData) error {
	matchStatus := Schedule.scheduleStatus(matchData)
	if matchStatus != Done || !isEditable(matchData) {
		return errors.New("the match is in the wrong state to delete the score")
	}

	matchData = Clone(matchData)

	currentScoreData := matchData.Sets()
	currentCourt := matchData.Court()
	courtOccupied := Courts.isOccupied(currentCourt)

	matchData.SetSets(nil)
	matchData.SetStartTime(types.DateTime{})
	matchData.SetEndTime(types.DateTime{})
	if courtOccupied {
		matchData.SetCourt(nil)
	}

	err := app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(matchData); err != nil {
			return err
		}
		for _, set := range currentScoreData {
			if err := txApp.Delete(set); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	match := Tournaments.matches[matchData.Id]

	match.Score = nil
	match.StartTime = time.Time{}
	match.EndTime = time.Time{}
	if courtOccupied {
		match.Location = nil
		Schedule.setMatchScheduleStatus(matchData, CourtWait, nil)
	} else {
		Schedule.setMatchScheduleStatus(matchData, Ready, currentCourt)
	}

	tournament := Tournaments.byMatch[matchData.Id]

	tournament.Update(nil)
	Schedule.updateTournamentScheduleStatus(tournament)

	return nil
}

func deleteScoreData(app core.App, matchData *MatchData) error {
	scoreData := matchData.Sets()
	for _, set := range scoreData {
		if err := app.Delete(set); err != nil {
			return err
		}
	}
	return nil
}

func isEditable(matchData *MatchData) bool {
	tournament := Tournaments.byMatch[matchData.Id]
	editable := tournament.EditableMatches()
	for _, m := range editable {
		editableMatchData := Tournaments.matchData[m.Id()]
		if editableMatchData.Id == matchData.Id {
			return true
		}
	}
	return false
}
