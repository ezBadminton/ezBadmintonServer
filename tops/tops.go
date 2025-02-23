// The tournament operations package contains
// the data structures holding the entire tournament state
// and exports mutexed functions for the api actions
package tops

import (
	"errors"
	"sync"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

var topsMu sync.RWMutex

var ErrUnexpected error = errors.New("an unexpected error occurred during tournament operations")

func ListTournaments() []*CompetitionTournament {
	defer topsMu.RUnlock()
	topsMu.RLock()

	return Tournaments.list
}

func StartTournament(app core.App, competitionId string) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return Tournaments.start(app, competitionId)
}

func StopTournament(app core.App, competitionId string) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return Tournaments.stop(app, competitionId)
}

func MakeDraw(app core.App, competition *Competition) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return makeDraw(app, competition)
}

func Redraw(app core.App, competition *Competition) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return redraw(app, competition)
}

func DrawSwap(app core.App, competition *Competition, a, b string) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return drawSwap(app, competition, a, b)
}

func DeleteDraw(app core.App, competition *Competition) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return deleteDraw(app, competition)
}

func SetSeeds(app core.App, competition *Competition, seeds []*Team) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return setSeeds(app, competition, seeds)
}

func ListRegistrations() []*Registration {
	defer topsMu.RUnlock()
	topsMu.RLock()

	return Registrations.list
}

func VerifyRegistration(team *Team, competition *Competition) error {
	defer topsMu.RUnlock()
	topsMu.RLock()

	return Registrations.verifyRegistration(team, competition)
}

func SetPlayerStatus(
	app core.App,
	player *Player,
	newStatus PlayerStatus,
	competitionIds []string,
) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return setPlayerStatus(app, player, newStatus, competitionIds)
}

func ListPlayerStatusChanges(player *Player, newStatus PlayerStatus) *StatusChangeResult {
	defer topsMu.RUnlock()
	topsMu.RLock()

	return listStatusChangeMatches(player, newStatus)
}

func AssignCourtToMatch(app core.App, matchData *MatchData, court *Court) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return Courts.assignCourtToMatch(app, matchData, court)
}

func UnassignCourt(app core.App, matchData *MatchData) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return Courts.unassignCourt(app, matchData)
}

func VerifyCourtDeletion(court *Court) error {
	topsMu.Lock()

	err := Courts.verifyCourtDeletion(court)
	if err != nil {
		defer topsMu.Unlock()
	}
	return err
}

func VerifyGymnasiumDeletion(gymnasium *Gymnasium) error {
	topsMu.Lock()

	err := Courts.verifyGymnasiumDeletion(gymnasium)
	if err != nil {
		defer topsMu.Unlock()
	}

	return err
}

func StartMatch(app core.App, matchData *MatchData) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return startMatch(app, matchData)
}

func CancelMatch(app core.App, matchData *MatchData) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return cancelMatch(app, matchData)
}

func SetMatchScore(app core.App, matchData *MatchData, score [][]int) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return setMatchScore(app, matchData, score)
}

func ResetMatch(app core.App, matchData *MatchData) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return resetMatch(app, matchData)
}
