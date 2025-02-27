// The tournament operations package contains
// the data structures holding the entire tournament state
// and exports mutexed functions for the api actions
package tops

import (
	"sync"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

type TournamentOperations struct {
	// All exported methods of the tournament operations
	// have to be sequentialized by this mutex lock to
	// guarantee correct state
	mu sync.RWMutex

	tournamentStore   *TournamentStore
	registrationStore *RegistrationStore
	courtStore        *CourtStore
	scheduler         *MatchScheduler
	matchManager      *MatchManager
	drawManager       *DrawManager
	withdrawalManager *WithdrawalManager
	tieBreakerManager *TieBreakerManager
}

func InitTournamentOperations(app core.App) {
	matchManager := newMatchManager()
	withdrawalManager := newWithdrawalManager()
	tieBreakerManager := newTieBreakerManager()
	courtStore := newCourtStore()
	registrationStore := newRegistrationStore(app, withdrawalManager)
	drawManager := newDrawManager(registrationStore)
	tournamentStore := newTournamentStore(app, drawManager, matchManager, courtStore, registrationStore, withdrawalManager, tieBreakerManager)
	playerTracker := newPlayerTracker(tournamentStore, courtStore, matchManager)
	scheduler := newMatchScheduler(app, tournamentStore, playerTracker, courtStore, matchManager)

	courtStore.init(scheduler, matchManager, tournamentStore)
	withdrawalManager.init(tournamentStore, registrationStore)

	tops = TournamentOperations{
		tournamentStore:   tournamentStore,
		registrationStore: registrationStore,
		courtStore:        courtStore,
		scheduler:         scheduler,
		matchManager:      matchManager,
		drawManager:       drawManager,
		withdrawalManager: withdrawalManager,
		tieBreakerManager: tieBreakerManager,
	}
}

var tops TournamentOperations

func ListTournaments() []*CompetitionTournament {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.tournamentStore.list
}

func StartTournament(app core.App, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.tournamentStore.start(app, competition)
}

func StopTournament(app core.App, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.tournamentStore.stop(app, competition)
}

func MakeDraw(app core.App, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.drawManager.makeDraw(app, competition)
}

func Redraw(app core.App, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.drawManager.redraw(app, competition)
}

func DrawSwap(app core.App, competition *Competition, a, b string) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.drawManager.drawSwap(app, competition, a, b)
}

func DeleteDraw(app core.App, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.drawManager.deleteDraw(app, competition)
}

func SetSeeds(app core.App, competition *Competition, seeds []*Team) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.drawManager.setSeeds(app, competition, seeds)
}

func ListRegistrations() []*Registration {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.registrationStore.list
}

func RegisterTeam(app core.App, team *Team, competition *Competition) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.registrationStore.registerTeam(app, team, competition)
}

func UpdateTeam(app core.App, team *Team) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.registrationStore.updateTeam(app, team)
}

func DeleteTeam(app core.App, team *Team) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.registrationStore.deleteTeam(app, team)
}

func SetPlayerStatus(
	app core.App,
	player *Player,
	newStatus PlayerStatus,
	competitions []*Competition,
) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.withdrawalManager.setPlayerStatus(app, player, newStatus, competitions)
}

func ListPlayerStatusChanges(player *Player, newStatus PlayerStatus) *StatusChangeResult {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.withdrawalManager.listStatusChangeMatches(player, newStatus)
}

func AssignCourtToMatch(app core.App, matchData *MatchData, court *Court) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.courtStore.assignCourtToMatch(app, matchData, court)
}

func UnassignCourt(app core.App, matchData *MatchData) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.courtStore.unassignCourt(app, matchData)
}

func DeleteCourt(e *core.RecordRequestEvent) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.courtStore.deleteCourt(e)
}

func DeleteGymnasium(e *core.RecordRequestEvent) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.courtStore.deleteGymnasium(e)
}

func StartMatch(app core.App, matchData *MatchData) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.matchManager.startMatch(app, matchData)
}

func CancelMatch(app core.App, matchData *MatchData) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.matchManager.cancelMatch(app, matchData)
}

func SetMatchScore(app core.App, matchData *MatchData, score [][]int) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.matchManager.setMatchScore(app, matchData, score)
}

func ResetMatch(app core.App, matchData *MatchData) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.matchManager.resetMatch(app, matchData)
}

func AddTieBreaker(app core.App, competition *Competition, teams []*Team) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.tieBreakerManager.addTieBreaker(app, competition, teams)
}

func UpdateTieBreaker(app core.App, tieBreaker *TieBreaker, teams []*Team) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.tieBreakerManager.updateTieBreaker(app, tieBreaker, teams)
}

func DeleteTieBreaker(app core.App, tieBreaker *TieBreaker) error {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	return tops.tieBreakerManager.deleteTieBreaker(app, tieBreaker)
}

func ListSchedule() []*Schedule {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.scheduler.listSchedule()
}

func ListScheduledRounds() []*ScheduledRound {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.scheduler.listScheduledRounds()
}
func ListScheduledMatches() []*ScheduledMatch {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.scheduler.listScheduledMatches()
}
