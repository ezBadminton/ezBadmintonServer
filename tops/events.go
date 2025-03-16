package tops

import (
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type BaseEvent struct {
	hook.Event
	*realtimeNotificationList
}

func newBaseEvent() BaseEvent {
	return BaseEvent{
		Event: hook.Event{},
		realtimeNotificationList: &realtimeNotificationList{
			notifications: make([]realtimeNotification, 0),
		},
	}
}

func (e BaseEvent) TriggerRealtimeNotifications() error {
	go e.realtimeNotificationList.TriggerRealtimeNotifications()
	return nil
}

type CompetitionEvent struct {
	BaseEvent

	App         core.App
	Competition *Competition
}

func newCompetitionEvent(app core.App, competition *Competition) *CompetitionEvent {
	return &CompetitionEvent{
		BaseEvent:   newBaseEvent(),
		App:         app,
		Competition: Clone(competition),
	}
}

func (e *CompetitionEvent) saveCompetition() error {
	return saveEventData(e.App, e, e.Competition)
}

func (e *CompetitionEvent) saveDeletedCompetition() error {
	if err := e.App.Delete(e.Competition); err != nil {
		return err
	}
	return e.Next()
}

func (e *CompetitionEvent) syncRegistrationParent(parent *RegistrationEvent) {
	parent.App = e.App
	parent.Competition = e.Competition
}

func (e *CompetitionEvent) syncToRegistrationParent(parent *RegistrationEvent) {
	e.App = parent.App
	e.Competition = parent.Competition
}

func (e *CompetitionEvent) syncSettingsParent(parent *TournamentModeSettingsEvent) {
	parent.App = e.App
	parent.Competition = e.Competition
}

func (e *CompetitionEvent) syncToSettingsParent(parent *TournamentModeSettingsEvent) {
	e.App = parent.App
	e.Competition = parent.Competition
}

type MatchEvent struct {
	BaseEvent

	App             core.App
	MatchData       *MatchData
	Match           *TournamentMatch
	ScheduleUpdates map[*CompetitionTournament]map[*TournamentMatch]struct{}
}

func newMatchEvent(app core.App, matchData *MatchData) *MatchEvent {
	return &MatchEvent{
		BaseEvent:       newBaseEvent(),
		App:             app,
		MatchData:       Clone(matchData),
		ScheduleUpdates: make(map[*CompetitionTournament]map[*TournamentMatch]struct{}),
	}
}

func (e *MatchEvent) saveMatchData() error {
	return saveEventData(e.App, e, e.MatchData)
}

type CourtEvent struct {
	*MatchEvent

	Court *Court
	// When the event is due to another match being reset
	// this is true.
	FromReset bool
}

func newCourtEvent(app core.App, matchData *MatchData, court *Court) *CourtEvent {
	return &CourtEvent{
		MatchEvent: newMatchEvent(app, matchData),
		Court:      court,
		FromReset:  false,
	}
}

type ScoreEvent struct {
	*MatchEvent

	ScoreData []*MatchSet
}

func newScoreEvent(app core.App, matchData *MatchData) *ScoreEvent {
	return &ScoreEvent{MatchEvent: newMatchEvent(app, matchData)}
}

func (e *ScoreEvent) saveScoreData() error {
	curScore := e.MatchData.Sets()
	err := e.App.RunInTransaction(func(txApp core.App) error {
		for _, s := range e.ScoreData {
			if err := txApp.Save(s); err != nil {
				return err
			}
		}
		e.MatchData.SetSets(e.ScoreData)
		if err := txApp.Save(e.MatchData); err != nil {
			return err
		}
		for _, s := range curScore {
			if err := txApp.Delete(s); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.Next()
}

type MatchResetEvent struct {
	*ScoreEvent

	// Matches that can no longer be ready because their
	// qualifications depend on the reset match
	DependantMatches []*ScheduledMatch
}

func newMatchResetEvent(app core.App, matchData *MatchData) *MatchResetEvent {
	return &MatchResetEvent{
		ScoreEvent:       newScoreEvent(app, matchData),
		DependantMatches: make([]*ScheduledMatch, 0),
	}
}

type RegistrationEvent struct {
	BaseEvent

	App          core.App
	Competition  *Competition
	Team         *Team
	Registration *Registration

	AddedPlayers, RemovedPlayers []*Player
}

func newRegistrationEvent(app core.App, competition *Competition, team *Team) *RegistrationEvent {
	return &RegistrationEvent{
		BaseEvent:   newBaseEvent(),
		App:         app,
		Competition: competition,
		Team:        team,
	}
}

func (e *RegistrationEvent) saveNewRegistration() error {
	err := e.App.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(e.Team); err != nil {
			return err
		}
		registrations := e.Competition.Registrations()
		registrations = append(registrations, e.Team)
		e.Competition.SetRegistrations(registrations)
		return txApp.Save(e.Competition)
	})
	if err != nil {
		return err
	}
	return e.Next()
}

func (e *RegistrationEvent) saveUpdatedRegistration() error {
	return saveEventData(e.App, e, e.Team)
}

func (e *RegistrationEvent) saveDeletedRegistration() error {
	if err := e.App.Delete(e.Team); err != nil {
		return err
	}
	return e.Next()
}

func (e *RegistrationEvent) syncParent(parent *CompetitionEvent) {
	parent.App = e.App
	parent.Competition = e.Competition
}

type TieBreakerEvent struct {
	*CompetitionEvent

	Teams      []*Team
	TieBreaker *TieBreaker
	GroupPhase *got.GroupPhase
}

func newTieBreakerEvent(app core.App, competition *Competition, teams []*Team) *TieBreakerEvent {
	return &TieBreakerEvent{
		CompetitionEvent: newCompetitionEvent(app, competition),
		Teams:            teams,
	}
}

func (e *TieBreakerEvent) saveNewTieBreaker() error {
	compTieBreakers := e.Competition.TieBreakers()
	err := e.App.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(e.TieBreaker); err != nil {
			return err
		}
		compTieBreakers = append(compTieBreakers, e.TieBreaker)
		e.Competition.SetTieBreakers(compTieBreakers)
		return txApp.Save(e.Competition)
	})
	if err != nil {
		return err
	}
	return e.Next()
}

func (e *TieBreakerEvent) saveUpdatedTieBreaker() error {
	if err := e.App.Save(e.TieBreaker); err != nil {
		return err
	}
	return e.Next()
}

func (e *TieBreakerEvent) saveDeletedTieBreaker() error {
	if err := e.App.Delete(e.TieBreaker); err != nil {
		return err
	}
	return e.Next()
}

type PlanEvent struct {
	*CompetitionEvent

	Tournament *CompetitionTournament
	MatchData  []*MatchData
}

func newPlanEvent(app core.App, competition *Competition, tournament *CompetitionTournament) *PlanEvent {
	return &PlanEvent{
		CompetitionEvent: newCompetitionEvent(app, competition),
		Tournament:       tournament,
	}
}

func (e *PlanEvent) saveStartedPlan() error {
	err := e.App.RunInTransaction(func(txApp core.App) error {
		for _, m := range e.MatchData {
			if err := txApp.Save(m); err != nil {
				return err
			}
		}
		e.Competition.SetMatches(e.MatchData)
		return txApp.Save(e.Competition)
	})
	if err != nil {
		return err
	}
	e.Tournament.Competition = e.Competition
	return e.Next()
}

func (e *PlanEvent) saveStoppedPlan() error {
	e.Competition.SetMatches(nil)
	err := e.App.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(e.Competition); err != nil {
			return err
		}
		for _, m := range e.MatchData {
			scoreData := m.Sets()
			if err := txApp.Delete(m); err != nil {
				return err
			}
			for _, s := range scoreData {
				if err := txApp.Delete(s); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.Next()
}

type StatusChangeEvent struct {
	BaseEvent

	App    core.App
	Player *Player
	Status PlayerStatus
	// Withdraw or reenter
	Withdraw bool
	// The competitions that are to be affected by the change
	Competitions     []*Competition
	Withdrawals      []*WithdrawEvent
	ChangedMatchData []*MatchData
}

func newStatusChangeEvent(
	app core.App,
	player *Player,
	status PlayerStatus,
	withdraw bool,
	competitions []*Competition,
) *StatusChangeEvent {
	return &StatusChangeEvent{
		BaseEvent:        newBaseEvent(),
		App:              app,
		Player:           Clone(player),
		Status:           status,
		Withdraw:         withdraw,
		Competitions:     competitions,
		Withdrawals:      make([]*WithdrawEvent, 0),
		ChangedMatchData: make([]*MatchData, 0),
	}
}

func (e *StatusChangeEvent) saveStatusChange() error {
	e.Player.SetStatus(e.Status)
	err := e.App.RunInTransaction(func(txApp core.App) error {
		for _, m := range e.ChangedMatchData {
			if err := txApp.Save(m); err != nil {
				return err
			}
		}
		return txApp.Save(e.Player)
	})
	if err != nil {
		return err
	}
	return e.Next()
}

type WithdrawEvent struct {
	hook.Event

	Competition *Competition
	// The event that caused the withdraw/reenter
	StatusChangeEvent *StatusChangeEvent
	// The registration that is being withdrawn/reentered
	Registration     *Registration
	WithdrawalPolicy got.WithdrawalPolicy
	// The matches that the registered team withdraws/reenters
	ChangedMatches []*got.Match
}

func newWithdrawEvent(competition *Competition, event *StatusChangeEvent) *WithdrawEvent {
	return &WithdrawEvent{
		Competition:       competition,
		StatusChangeEvent: event,
	}
}

type SettingsEvent struct {
	hook.Event

	App         core.App
	OldSettings *TournamentEvent
	NewSettings *TournamentEvent
}

func newSettingsEvent(parent *core.RecordRequestEvent, old, new *TournamentEvent) *SettingsEvent {
	return &SettingsEvent{
		Event:       hook.Event{},
		App:         parent.App,
		OldSettings: old,
		NewSettings: new,
	}
}

func (e *SettingsEvent) syncRequest(re *core.RecordRequestEvent) {
	re.App = e.App
}

func (e *SettingsEvent) syncToRequest(re *core.RecordRequestEvent) {
	e.App = re.App
}

type PlayerRestEvent struct {
	*MatchEvent

	Players    []*Player
	Match      *TournamentMatch
	Tournament *CompetitionTournament
}

func newPlayerRestEvent(players []*Player, match *TournamentMatch) *PlayerRestEvent {
	return &PlayerRestEvent{
		MatchEvent: &MatchEvent{
			BaseEvent:       newBaseEvent(),
			Match:           match,
			ScheduleUpdates: make(map[*CompetitionTournament]map[*TournamentMatch]struct{}),
		},
		Players: players,
		Match:   match,
	}
}

// The rest time changed and the rest period of the
// MatchData got extended/reduced. Matches that remain
// in their rest period before and after the change are
// not listed.
type MatchRestEvent struct {
	BaseEvent

	Matches         []*TournamentMatch
	ScheduleUpdates map[*CompetitionTournament]map[*TournamentMatch]struct{}
}

func newMatchRestEvent(matches []*TournamentMatch) *MatchRestEvent {
	return &MatchRestEvent{
		BaseEvent:       newBaseEvent(),
		Matches:         matches,
		ScheduleUpdates: make(map[*CompetitionTournament]map[*TournamentMatch]struct{}),
	}
}

type RescheduleEvent struct {
	hook.Event

	Schedule           *Schedule
	StartedTournaments []*CompetitionTournament
}

func newRescheduleEvent(schedule *Schedule) *RescheduleEvent {
	return &RescheduleEvent{
		Event:    hook.Event{},
		Schedule: schedule,
	}
}

type ScheduleStatusEvent struct {
	hook.Event

	Match          *got.Match
	MatchData      *MatchData
	Competition    *Competition
	Status         ScheduleStatus
	BlockingStatus map[string]PlayerBlock

	PlayersInMatch map[*Player]*MatchData
	PlayersResting map[*Player]time.Time
}

func newScheduleStatusEvent(match *got.Match, competition *Competition) *ScheduleStatusEvent {
	return &ScheduleStatusEvent{
		Event:       hook.Event{},
		Match:       match,
		Competition: competition,
		Status:      -1,
	}
}

type CategorizationEvent struct {
	hook.Event

	App                  core.App
	UseAgeGroups         bool
	UsePlayingLevels     bool
	AgeGroupsFlipped     bool
	PlayingLevelsFlipped bool
	Competitions         []*Competition
}

func newCategorizationEvent(
	app core.App,
	useAgeGroups,
	usePlayingLevels,
	ageGroupsFlipped,
	playingLevelsFlipped bool,
	competitions []*Competition,
) *CategorizationEvent {
	return &CategorizationEvent{
		Event:                hook.Event{},
		App:                  app,
		UseAgeGroups:         useAgeGroups,
		UsePlayingLevels:     usePlayingLevels,
		AgeGroupsFlipped:     ageGroupsFlipped,
		PlayingLevelsFlipped: playingLevelsFlipped,
		Competitions:         competitions,
	}
}

func (e *CategorizationEvent) syncSettingsParent(parent *SettingsEvent) {
	parent.App = e.App
}

func (e *CategorizationEvent) syncToSettingsParent(parent *SettingsEvent) {
	e.App = parent.App
}

func (e *CategorizationEvent) syncDeleteParent(parent *CategoryDeleteEvent) {
	parent.App = e.App
}

func (e *CategorizationEvent) syncToDeleteParent(parent *CategoryDeleteEvent) {
	e.App = parent.App
}

type CategoryDeleteEvent struct {
	hook.Event

	App         core.App
	Category    Category
	Replacement Category
}

func newCategoryDeleteEvent(app core.App, category, replacement Category) *CategoryDeleteEvent {
	return &CategoryDeleteEvent{
		Event:       hook.Event{},
		App:         app,
		Category:    category,
		Replacement: replacement,
	}
}

func (e *CategoryDeleteEvent) syncParent(parent *core.RecordRequestEvent) {
	parent.App = e.App
}

func (e *CategoryDeleteEvent) syncToParent(parent *core.RecordRequestEvent) {
	e.App = parent.App
}

type TournamentModeSettingsEvent struct {
	hook.Event

	App         core.App
	Settings    *TournamentModeSettings
	Competition *Competition
}

func newTournamentModeSettingsEvent(app core.App, settings *TournamentModeSettings, competition *Competition) *TournamentModeSettingsEvent {
	return &TournamentModeSettingsEvent{
		Event:       hook.Event{},
		App:         app,
		Settings:    settings,
		Competition: competition,
	}
}

func saveEventData(app core.App, e hook.Resolver, dataProxy core.RecordProxy) error {
	if err := app.Save(dataProxy.ProxyRecord()); err != nil {
		return err
	}
	return e.Next()
}
