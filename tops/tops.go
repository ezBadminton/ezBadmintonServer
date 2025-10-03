// The tournament operations package contains
// the data structures holding the entire tournament state
// and exports mutexed functions for the api actions
package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

type TournamentOperations struct {
	*UnitOfWorkManager

	tournamentStore               *TournamentStore
	registrationStore             *RegistrationStore
	courtStore                    *CourtStore
	scheduler                     *MatchScheduler
	matchManager                  *MatchManager
	drawManager                   *DrawManager
	withdrawalManager             *WithdrawalManager
	tieBreakerManager             *TieBreakerManager
	qualificationOverrideManager  *QualificationOverrideManager
	eventSettingsManager          *EventSettingsManager
	playerTracker                 *PlayerTracker
	categorizationManager         *CategorizationManager
	tournamentModeSettingsManager *TournamentModeSettingsManager
	competitionManager            *CompetitionManager
	startingFeeManager            *StartingFeeManager
}

func InitTournamentOperations(app core.App) {
	unitOfWorkManager := newUnitOfWorkManager(app)
	withdrawalManager := newWithdrawalManager()
	tieBreakerManager := newTieBreakerManager()
	qualificationOverrideManager := newQualificationOverrideManager()
	courtStore := newCourtStore()
	eventSettingsManager := newEventSettingsManager()
	tournamentModeSettingsManager := newTournamentModeSettingsManager()
	competitionManager := newCompetitionManager()
	playerTracker := newPlayerTracker()
	startingFeeManager := newStartingFeeManager(app)
	matchManager := newMatchManager(withdrawalManager)
	scheduler := newMatchScheduler(app)
	categorizationManager := newCategorizationManager(eventSettingsManager)
	registrationStore := newRegistrationStore(
		app,
		withdrawalManager,
		competitionManager,
		qualificationOverrideManager,
		startingFeeManager,
	)
	drawManager := newDrawManager(registrationStore, tournamentModeSettingsManager)
	tournamentStore := newTournamentStore(
		app,
		drawManager,
		matchManager,
		courtStore,
		registrationStore,
		withdrawalManager,
		tieBreakerManager,
		qualificationOverrideManager,
		scheduler,
		playerTracker,
		categorizationManager,
		tournamentModeSettingsManager,
		competitionManager,
	)

	playerTracker.init(tournamentStore, courtStore, matchManager, eventSettingsManager, scheduler)
	scheduler.init(tournamentStore, courtStore, matchManager, playerTracker)
	courtStore.init(scheduler, matchManager, withdrawalManager, tournamentStore)
	withdrawalManager.init(tournamentStore, registrationStore)

	tops = TournamentOperations{
		UnitOfWorkManager:             unitOfWorkManager,
		tournamentStore:               tournamentStore,
		registrationStore:             registrationStore,
		courtStore:                    courtStore,
		scheduler:                     scheduler,
		matchManager:                  matchManager,
		drawManager:                   drawManager,
		withdrawalManager:             withdrawalManager,
		tieBreakerManager:             tieBreakerManager,
		qualificationOverrideManager:  qualificationOverrideManager,
		eventSettingsManager:          eventSettingsManager,
		playerTracker:                 playerTracker,
		categorizationManager:         categorizationManager,
		tournamentModeSettingsManager: tournamentModeSettingsManager,
		competitionManager:            competitionManager,
		startingFeeManager:            startingFeeManager,
	}
}

var tops TournamentOperations

func ListTournaments() []*CompetitionTournament {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.tournamentStore.list
}

func ListMatches() []*TournamentMatch {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.tournamentStore.listMatches()
}

func StartTournament(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tournamentStore.start(app, competition)
}

func StopTournament(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tournamentStore.stop(app, competition)
}

func MakeDraw(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.drawManager.makeDraw(app, competition)
}

func Redraw(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.drawManager.redraw(app, competition)
}

func DrawSwap(app core.App, competition *Competition, a, b string) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.drawManager.drawSwap(app, competition, a, b)
}

func DeleteDraw(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.drawManager.deleteDraw(app, competition)
}

func SetSeeds(app core.App, competition *Competition, seeds []*Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.drawManager.setSeeds(app, competition, seeds)
}

func QualificationOverrideSwap(app core.App, competition *Competition, a, b *Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.qualificationOverrideManager.overrideSwap(app, competition, a, b)

}
func QualificationOverrideReset(app core.App, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.qualificationOverrideManager.overrideReset(app, competition)
}

func ListRegistrations() []*Registration {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.registrationStore.list
}

func RegisterTeam(app core.App, team *Team, competition *Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.registrationStore.registerTeam(app, team, competition)
}

func UpdateTeam(app core.App, team *Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.registrationStore.updateTeam(app, team)
}

func DeleteTeam(app core.App, team *Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.registrationStore.deleteTeam(app, team)
}

func SetPlayerStatus(
	app core.App,
	player *Player,
	newStatus PlayerStatus,
	competitions []*Competition,
) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.withdrawalManager.setPlayerStatus(app, player, newStatus, competitions)
}

func ListPlayerStatusChanges(player *Player, newStatus PlayerStatus) *StatusChangeResult {
	defer tops.mu.RUnlock()
	tops.mu.RLock()

	return tops.withdrawalManager.listStatusChangeMatches(player, newStatus)
}

func AssignCourtToMatch(app core.App, matchData *MatchData, court *Court) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.courtStore.assignCourtToMatch(app, matchData, court)
}

func UnassignCourt(app core.App, matchData *MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.courtStore.unassignCourt(app, matchData)
}

func DeleteCourt(e *core.RecordRequestEvent) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.courtStore.deleteCourt(e)
}

func DeleteGymnasium(e *core.RecordRequestEvent) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.courtStore.deleteGymnasium(e)
}

func StartMatch(app core.App, matchData *MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.matchManager.startMatch(app, matchData)
}

func CancelMatch(app core.App, matchData *MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.matchManager.cancelMatch(app, matchData)
}

func EndMatch(app core.App, matchData *MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.matchManager.endMatch(app, matchData)
}

func SetMatchScore(app core.App, matchData *MatchData, score [][]int) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.matchManager.setMatchScore(app, matchData, score)
}

func ResetMatch(app core.App, matchData *MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.matchManager.resetMatch(app, matchData)
}

func AddTieBreaker(app core.App, competition *Competition, teams []*Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tieBreakerManager.addTieBreaker(app, competition, teams)
}

func UpdateTieBreaker(app core.App, tieBreaker *TieBreaker, teams []*Team) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tieBreakerManager.updateTieBreaker(app, tieBreaker, teams)
}

func DeleteTieBreaker(app core.App, tieBreaker *TieBreaker) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

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

func ChangeEventSettings(e *core.RecordRequestEvent) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.eventSettingsManager.changeSettings(e)
}

func DeletePlayerFromTeam(app core.App, player *Player) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.registrationStore.handleTeamPlayerDeletion(app, player)
}

func DeleteCategory(e *core.RecordRequestEvent) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.categorizationManager.deleteCategory(e)
}

func AddPlayingLevel(e *core.RecordRequestEvent) error {
	return tops.categorizationManager.addPlayingLevel(e)
}

func DeletePlayingLevel(e *core.RecordRequestEvent) error {
	return tops.categorizationManager.deletePlayingLevel(e)
}

func ReorderPlayingLevel(app core.App, from, to int) error {
	return tops.categorizationManager.reorderPlayingLevel(app, from, to)
}

func SetTournamentModeSettings(e *core.RecordRequestEvent) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tournamentModeSettingsManager.setSettings(e)
}

func DeleteCompetitions(app core.App, competitions []*Competition) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.competitionManager.deleteCompetitions(app, competitions)
}

func MarkMatchSheetsAsPrinted(app core.App, matchData []*MatchData) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.tournamentStore.markMatchSheetsAsPrinted(app, matchData)
}

func BulkSetPlayerStatus(app core.App, players []*Player, status PlayerStatus) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return bulkSetPlayerStatus(app, players, status)
}

func CreateTestPlayers(app core.App, amount int) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return createTestPlayers(app, amount)
}

func SetStartingFees(competitions []*Competition, startingFee int) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.startingFeeManager.setStartingFees(competitions, startingFee)
}

func PayStartingFee(player *Player, amount, discountPercent int) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.startingFeeManager.payFee(player, amount, discountPercent)
}

func UpdateOrCreateMassDiscount(minRegistrations, amount int) error {
	defer tops.endUnitOfWork()
	tops.startUnitOfWork()

	return tops.startingFeeManager.updateOrCreateMassDiscount(minRegistrations, amount)
}
