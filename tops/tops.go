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

func CreateTournament(competition *Competition) (*CompetitionTournament, error) {
	topsMu.Lock()

	tournament, err := Tournaments.createTournament(competition)
	if err != nil {
		// Only unlock on error. Otherwise the competition update handler unlocks after transaction.
		defer topsMu.Unlock()
	}
	return tournament, err
}

func MakeDraw(app core.App, competition *Competition) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return makeDraw(app, competition)
}

func DrawSwap(app core.App, competition *Competition, a, b string) error {
	defer topsMu.Unlock()
	topsMu.Lock()

	return drawSwap(app, competition, a, b)
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

func WithdrawOrReenterPlayer(
	app core.App,
	player *Player,
	competitionIds []string,
	withdraw bool,
) ([]*FloatingStatusChange, error) {
	topsMu.Lock()

	changes, err := withdrawOrReenterPlayer(app, player, competitionIds, withdraw)
	if err != nil {
		// Only unlock on error. Otherwise the player update handler unlocks after transaction.
		defer topsMu.Unlock()
	}

	return changes, err
}

func ListPlayerStatusChanges(player *Player, newStatus PlayerStatus) *StatusChangeResult {
	defer topsMu.RUnlock()
	topsMu.RLock()

	return listStatusChangeMatches(player, newStatus)
}
