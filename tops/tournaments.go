package tops

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/gotournament/badminton"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type Tournament interface {
	got.RankingUpdater
	got.MatchLister
	got.WithdrawalPolicy
	got.EditingPolicy

	ToMap(getMatchId func(int) string) map[string]any
}

type TournamentMatch struct {
	BaseTopsRecord
	match     *got.Match
	matchData *MatchData
}

func (m *TournamentMatch) ToMap() map[string]any {
	result := m.match.ToMap()
	if m.matchData != nil {
		matchData := m.matchData.Record.FieldsData()
		maps.Copy(result, matchData)
	}
	return m.BaseTopsRecord.ToMap(result)
}

type CompetitionTournament struct {
	BaseTopsRecord
	*Competition
	Tournament
	badminton.ScoreSettings
	Started, Ended bool
}

func (c *CompetitionTournament) ToMap() map[string]any {
	comp := c.Competition
	matchIdGetter := createMatchIdGetter(comp.Id)
	tournament := c.Tournament.ToMap(matchIdGetter)
	data := map[string]any{
		"competition": c.Competition.Id,
		"tournament":  tournament,
		"started":     c.Started,
		"ended":       c.Ended,
	}
	return c.BaseTopsRecord.ToMap(data)
}

type TournamentStore struct {
	app core.App

	list []*CompetitionTournament
	// match id -> tournament match
	matches map[string]*TournamentMatch
	// competition id -> match list
	competitionMatches map[string][]*TournamentMatch
	// competition id -> tournament
	tournaments map[string]*CompetitionTournament
	// got match id -> match
	hydratedMatches map[int]*TournamentMatch
	// match id -> tournament
	byMatch map[string]*CompetitionTournament
	// competition id -> player id -> matches
	byCompetitionPlayer map[string]map[string][]*TournamentMatch

	// Before match data create. After e.Next() the match data has been persisted and the tournament hydrated
	onStart *hook.Hook[*PlanEvent]
	// Before match data delete. After e.Next() the match data deletion has been persisted.
	onStop *hook.Hook[*PlanEvent]
	// Before tournament plan update. After e.Next() the tournament has been updated
	onUpdate *hook.Hook[*PlanEvent]
}

func newTournamentStore(
	app core.App,
	drawManager *DrawManager,
	matchManager *MatchManager,
	courtStore *CourtStore,
	registrationStore *RegistrationStore,
	withdrawalManager *WithdrawalManager,
	tieBreakerManager *TieBreakerManager,
	scheduler *MatchScheduler,
	playerTracker *PlayerTracker,
	categorizationManager *CategorizationManager,
	tournamentModeSettingsManager *TournamentModeSettingsManager,
	competitionManager *CompetitionManager,
) *TournamentStore {
	compStore, _ := store.FindRecordStore[Competition]()

	s := &TournamentStore{
		app:                 app,
		tournaments:         make(map[string]*CompetitionTournament),
		matches:             make(map[string]*TournamentMatch),
		competitionMatches:  make(map[string][]*TournamentMatch),
		byCompetitionPlayer: make(map[string]map[string][]*TournamentMatch),
		list:                make([]*CompetitionTournament, 0),
		hydratedMatches:     make(map[int]*TournamentMatch),
		byMatch:             make(map[string]*CompetitionTournament),
		onStart:             &hook.Hook[*PlanEvent]{},
		onStop:              &hook.Hook[*PlanEvent]{},
		onUpdate:            &hook.Hook[*PlanEvent]{},
	}

	err := s.addTournaments(compStore.RecordList...)
	if err != nil {
		panic("unable to initialize tournament store")
	}

	drawManager.onDraw.BindFunc(s.verifyDrawChange)
	drawManager.onDraw.BindFunc(s.handleDraw)
	drawManager.onDrawDelete.BindFunc(s.handleDrawDeletion)
	drawManager.onDrawSwap.BindFunc(s.verifyDrawChange)
	drawManager.onDrawSwap.BindFunc(s.handleDraw)
	drawManager.onSetSeeds.BindFunc(s.verifyDrawChange)

	// Priority for setting the e.Match
	courtStore.onCourtAssign.Bind(priorityHandler(s.handleCourtAssignment, -1))
	courtStore.onCourtUnassign.Bind(priorityHandler(s.handleCourtAssignment, -1))
	// Priority for setting the e.RelatedByPlayers
	courtStore.onCourtAssign.Bind(priorityHandler(s.prepareCourtAssignment, 1))
	courtStore.onCourtUnassign.Bind(priorityHandler(s.prepareCourtAssignment, 1))

	matchManager.onStart.BindFunc(s.handleMatchStart)
	matchManager.onCancel.BindFunc(s.handleMatchCancel)
	// Priority before player tracker
	matchManager.onScoreSet.Bind(priorityHandler(s.handleScoreSet, 2))
	matchManager.onReset.BindFunc(s.handleMatchReset)

	registrationStore.onUpdate.BindFunc(s.verifyRegistrationUpdate)
	registrationStore.onDelete.BindFunc(s.verifyUnregistration)

	// Priority for settings e.WithdrawalPolicy
	withdrawalManager.onWithdraw.Bind(priorityHandler(s.verifyWithdrawal, -1))
	withdrawalManager.onStatusChange.BindFunc(s.handleStatusChange)

	tieBreakerManager.onAdd.BindFunc(s.verifyTieBreakerAdd)
	tieBreakerManager.onAdd.BindFunc(s.verifyTieBreakerChange)
	tieBreakerManager.onAdd.BindFunc(s.handleTieBreakerChange)
	tieBreakerManager.onUpdate.BindFunc(s.verifyTieBreakerChange)
	tieBreakerManager.onUpdate.BindFunc(s.handleTieBreakerChange)
	tieBreakerManager.onDelete.BindFunc(s.verifyTieBreakerChange)
	tieBreakerManager.onDelete.BindFunc(s.handleTieBreakerDelete)

	// Priority for setting the started tournament(s)/match data in the events
	scheduler.onReschedule.Bind(priorityHandler(s.handleReschedule, -1))
	scheduler.onStatusChange.Bind(priorityHandler(s.handleScheduleStatus, -1))

	// Priority for setting the tournament in the events
	playerTracker.onRestEnd.Bind(priorityHandler(s.handleRestEnd, -1))
	playerTracker.onRestChanged.Bind(priorityHandler(s.handleRestSettingsChange, -1))

	categorizationManager.onCategorizationChange.BindFunc(s.verifyCategorizationChange)
	categorizationManager.onCategoryDelete.BindFunc(s.verifyCategoryDelete)

	// Priority for verifying before the update deletes a draw (DrawManager)
	tournamentModeSettingsManager.onSet.Bind(priorityHandler(s.verifyModeSettingsUpdate, -1))

	// Priority for verifying before the delete deletes registrations
	competitionManager.onDelete.Bind(priorityHandler(s.handleCompetitionDelete, -1))

	return s
}

func (s *TournamentStore) listStarted() []*CompetitionTournament {
	tournaments := make([]*CompetitionTournament, 0, len(s.list))
	for _, t := range s.list {
		if t.Started {
			tournaments = append(tournaments, t)
		}
	}

	return tournaments
}

func (s *TournamentStore) listMatches() []*TournamentMatch {
	matches := make([]*TournamentMatch, 0)
	for _, competitionMatches := range s.competitionMatches {
		matches = append(matches, competitionMatches...)
	}
	return matches
}

func (s *TournamentStore) setTournament(realtimeNotifier realtimeNotifier, competition *Competition, tournament *CompetitionTournament) {
	_, isUpdate := s.tournaments[competition.Id]

	s.tournaments[competition.Id] = tournament

	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competition.Id
	})
	s.list = append(s.list, tournament)
	slices.SortFunc(s.list, compareTournaments)

	if isUpdate {
		s.updateTournamentMatches(realtimeNotifier, tournament)
	} else {
		s.createTournamentMatches(realtimeNotifier, tournament)
	}
}

func (s *TournamentStore) update(realtimeNotifier realtimeNotifier, tournament *CompetitionTournament) {
	event := newPlanEvent(s.app, tournament.Competition, tournament)
	s.onUpdate.Trigger(event,
		func(e *PlanEvent) error {
			e.Tournament.Update(nil)
			matches := s.competitionMatches[e.Tournament.Competition.Id]
			s.updatePlayerMatchMap(tournament.Competition)
			s.realtimeUpdateNotification(realtimeNotifier, e.Tournament, matches)
			return e.Next()
		},
	)
	event.TriggerRealtimeNotifications()
}

func (s *TournamentStore) realtimeUpdateNotification(realtimeNotifier realtimeNotifier, tournament *CompetitionTournament, matches []*TournamentMatch) {
	for _, m := range matches {
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeUpdate, m, 0)
	}
	realtimeNotifier.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeUpdate, tournament, 0)
}

func (s *TournamentStore) removeTournament(realtimeNotifier realtimeNotifier, competition *Competition) {
	tournament := s.tournaments[competition.Id]
	matches := tournament.MatchList().Matches
	matchIdGetter := createMatchIdGetter(competition.Id)
	for i, m := range matches {
		delete(s.hydratedMatches, m.Id())
		matchId := matchIdGetter(i)
		delete(s.byMatch, matchId)
		delete(s.matches, matchId)
	}
	tMatches := s.competitionMatches[competition.Id]
	delete(s.competitionMatches, competition.Id)
	delete(s.byCompetitionPlayer, competition.Id)

	delete(s.tournaments, competition.Id)
	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competition.Id
	})

	realtimeNotifier.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeDelete, tournament, 0)
	for _, m := range tMatches {
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeDelete, m, 0)
	}
}

func (s *TournamentStore) initTournamentMatches(tournament *CompetitionTournament) {
	matchDataList := tournament.Competition.Matches()
	matches := tournament.MatchList().Matches
	matchIdGetter := createMatchIdGetter(tournament.Competition.Id)
	tMatches := make([]*TournamentMatch, len(matches))
	for i, m := range matches {
		var matchData *MatchData
		if len(matchDataList) > 0 {
			matchData = matchDataList[i]
		}
		id := matchIdGetter(i)
		tMatch := &TournamentMatch{
			BaseTopsRecord: BaseTopsRecord{Id: id},
			match:          m,
			matchData:      matchData,
		}
		s.matches[id] = tMatch
		s.hydratedMatches[m.Id()] = tMatch
		tMatches[i] = tMatch
	}
	s.competitionMatches[tournament.Competition.Id] = tMatches
	s.updatePlayerMatchMap(tournament.Competition)
}

func (s *TournamentStore) createTournamentMatches(realtimeNotifier realtimeNotifier, tournament *CompetitionTournament) {
	s.initTournamentMatches(tournament)
	tMatches := s.competitionMatches[tournament.Competition.Id]
	for _, match := range tMatches {
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeCreate, match, 0)
	}
	realtimeNotifier.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeCreate, tournament, 0)
}

func (s *TournamentStore) updateTournamentMatches(realtimeNotifier realtimeNotifier, tournament *CompetitionTournament) {
	oldTMatches := s.competitionMatches[tournament.Competition.Id]
	s.initTournamentMatches(tournament)
	tMatches := s.competitionMatches[tournament.Competition.Id]
	numOldMatches, numMatches := len(oldTMatches), len(tMatches)
	matchDelta := numMatches - numOldMatches
	for i := range max(0, matchDelta) {
		addedMatch := tMatches[numOldMatches+i]
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeCreate, addedMatch, 0)
	}
	for i := range max(0, -matchDelta) {
		removedMatch := oldTMatches[numMatches+i]
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeDelete, removedMatch, 0)
	}
	for i := range numMatches + min(0, -matchDelta) {
		updatedMatch := tMatches[i]
		realtimeNotifier.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeUpdate, updatedMatch, 0)
	}
	realtimeNotifier.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeUpdate, tournament, 0)
}

func (s *TournamentStore) updatePlayerMatchMap(competition *Competition) {
	tMatches := s.competitionMatches[competition.Id]
	playerMatches := make(map[string][]*TournamentMatch)
	for _, match := range tMatches {
		for _, p := range playersInMatch(match.match) {
			_, ok := playerMatches[p.Id]
			if !ok {
				playerMatches[p.Id] = make([]*TournamentMatch, 0)
			}
			playerMatches[p.Id] = append(playerMatches[p.Id], match)
		}
	}
	s.byCompetitionPlayer[competition.Id] = playerMatches
}

func (s *TournamentStore) addTournaments(competitions ...*Competition) error {
	for _, comp := range competitions {
		tournament, err := s.createTournament(comp)
		if errors.Is(err, ErrNoDraw) {
			continue
		}
		if err != nil {
			return err
		}
		s.initTournamentMatches(tournament)
		if err := s.hydrate(tournament); err != nil {
			return err
		}
		s.tournaments[comp.Id] = tournament
		s.list = append(s.list, tournament)
	}
	slices.SortFunc(s.list, compareTournaments)
	return nil
}

func (s *TournamentStore) createTournament(comp *Competition) (*CompetitionTournament, error) {
	currentTournament, ok := s.tournaments[comp.Id]
	if ok && currentTournament.Started {
		return nil, errors.New("can not update draw while competition is running")
	}

	entries, err := newEntries(comp)
	if err != nil {
		return nil, err
	}

	settings := comp.TournamentModeSettings()
	if settings == nil {
		return nil, errors.New("cannot create tournament without mode settings")
	}

	scoreSettings, err := newScoreSettings(settings)
	if err != nil {
		return nil, err
	}

	var tournament Tournament
	switch settings.Type() {
	case SingleElimination:
		tournament, err = got.NewSingleElimination(entries)
	case SingleEliminationWithConsolation:
		tournament, err = got.NewSingleEliminationWithConsolation(
			entries,
			settings.NumConsolationRounds(),
			settings.PlacesToPlayOut(),
		)
	case RoundRobin:
		tournament, err = got.NewRoundRobin(
			entries,
			settings.Passes(),
			badminton.MaxScore(scoreSettings),
		)
	case GroupKnockout:
		tournament, err = got.NewGroupKnockout(
			entries,
			knockoutBuilder(settings),
			settings.NumGroups(),
			settings.NumQualifications(),
			badminton.MaxScore(scoreSettings),
		)
	case DoubleElimination:
		tournament, err = got.NewDoubleElimination(entries)
	default:
		panic("unknown tournament type")
	}

	if err != nil {
		return nil, err
	}

	id := "t-" + comp.Id
	created := comp.Created()
	updated := comp.Updated()

	compTournament := &CompetitionTournament{
		BaseTopsRecord: BaseTopsRecord{
			Id:      id,
			Created: created,
			Updated: updated,
		},
		Competition:   comp,
		Tournament:    tournament,
		ScoreSettings: scoreSettings,
	}

	return compTournament, nil
}

func (s *TournamentStore) start(app core.App, competition *Competition) error {
	tournament := s.tournaments[competition.Id]
	if tournament == nil {
		return ErrNoDraw
	}
	if tournament.Started {
		return errors.New("competition already running")
	}

	event := newPlanEvent(app, competition, tournament)
	err := s.onStart.Trigger(event,
		s.startHandler,
		(*PlanEvent).saveStartedPlan,
		s.hydrateHandler,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
}

func (s *TournamentStore) startHandler(e *PlanEvent) error {
	matchData, err := createMatchData(e.App, e.Tournament)
	if err != nil {
		return err
	}
	e.MatchData = matchData
	return e.Next()
}

func (s *TournamentStore) hydrateHandler(e *PlanEvent) error {
	s.hydrate(e.Tournament)
	e.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeUpdate, e.Tournament, 0)
	return e.Next()
}

func (s *TournamentStore) stop(app core.App, competition *Competition) error {
	tournament := s.tournaments[competition.Id]
	if tournament == nil {
		return ErrNoDraw
	}
	if !tournament.Started {
		return errors.New("competition is not running")
	}
	event := newPlanEvent(app, competition, tournament)
	err := s.onStop.Trigger(event,
		s.stopHandler,
		(*PlanEvent).saveStoppedPlan,
		s.dehydrateHandler,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
}

func (s *TournamentStore) stopHandler(e *PlanEvent) error {
	e.MatchData = e.Competition.Matches()
	return e.Next()
}

func (s *TournamentStore) dehydrateHandler(e *PlanEvent) error {
	s.dehydrate(e.Tournament)
	s.realtimeUpdateNotification(e, e.Tournament, s.competitionMatches[e.Competition.Id])
	return e.Next()
}

func (s *TournamentStore) verifyDrawChange(e *CompetitionEvent) error {
	tournament := s.tournaments[e.Competition.Id]
	if tournament != nil && tournament.Started {
		return errors.New("tournament is already running. Can not make draw or seeding changes")
	}
	return e.Next()
}

func (s *TournamentStore) handleDraw(e *CompetitionEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	tournament, err := s.createTournament(e.Competition)
	if err != nil {
		return err
	}

	s.setTournament(e, e.Competition, tournament)
	return nil
}

func (s *TournamentStore) handleDrawDeletion(e *CompetitionEvent) error {
	tournament := s.tournaments[e.Competition.Id]
	if tournament == nil {
		return errors.New("this competition has no draw. Can not delete draw")
	}
	if tournament.Started {
		return errors.New("can not delete draw of running tournament")
	}
	if err := e.Next(); err != nil {
		return err
	}
	s.removeTournament(e, e.Competition)
	return nil
}

func (s *TournamentStore) handleMatchStart(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	match := s.matches[e.MatchData.Id]
	match.matchData = e.MatchData
	match.match.StartTime = e.MatchData.StartTime().Time()

	tournament := s.byMatch[e.MatchData.Id]
	tournament.UpdateEditableMatches()
	e.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeUpdate, match, 0)
	e.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeUpdate, tournament, 0)
	return nil
}

func (s *TournamentStore) handleMatchCancel(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	match := s.matches[e.MatchData.Id]
	match.matchData = e.MatchData
	match.match.StartTime = time.Time{}

	tournament := s.byMatch[e.MatchData.Id]
	tournament.UpdateEditableMatches()
	e.AddRealtimeNotification(s.app, "tournament_matches", core.ModelEventTypeUpdate, match, 0)
	e.AddRealtimeNotification(s.app, "tournament_plans", core.ModelEventTypeUpdate, tournament, 0)
	return nil
}

func (s *TournamentStore) handleScoreSet(e *ScoreEvent) error {
	if !e.MatchData.EndTime().IsZero() && !s.isEditable(e.MatchData) {
		return errors.New("the match is not in an editable state")
	}

	e.Match = s.matches[e.MatchData.Id]
	tournament := s.byMatch[e.MatchData.Id]
	score, err := scoreDataToScore(e.ScoreData, tournament.ScoreSettings)
	if err != nil {
		return err
	}
	matchEnding := e.MatchData.EndTime().IsZero()

	if err := e.Next(); err != nil {
		return err
	}

	e.Match.matchData = e.MatchData
	e.Match.match.Score = score
	if matchEnding {
		e.Match.match.EndTime = e.MatchData.EndTime().Time()
		tournament.Ended = matchesFinished(tournament.MatchList().Matches)
	}
	s.update(e, tournament)

	e.MatchEvent.ScheduleUpdates = s.collectPlayerMatches(e.MatchEvent.Match.match)

	return nil
}

func (s *TournamentStore) handleMatchReset(e *MatchResetEvent) error {
	if !s.isEditable(e.MatchData) {
		return errors.New("the match is not in an editable state")
	}

	if err := e.Next(); err != nil {
		return err
	}

	match := s.matches[e.MatchData.Id]
	match.matchData = e.MatchData
	match.match.Score = nil
	match.match.StartTime = time.Time{}
	match.match.EndTime = time.Time{}
	if e.MatchData.Court() == nil {
		match.match.Location = nil
	}

	tournament := s.byMatch[e.MatchData.Id]
	tournament.Ended = false
	s.update(e, tournament)

	filteredDependantMatches := make([]*ScheduledMatch, 0)
	for _, match := range e.DependantMatches {
		matchTournament := s.byMatch[match.Match.Id]
		if matchTournament.Id != tournament.Id {
			continue
		}
		tournamentMatch := s.matches[match.Match.Id]
		players := playersInMatch(tournamentMatch.match)
		if len(players) < 2*tournament.Competition.TeamSize() {
			filteredDependantMatches = append(filteredDependantMatches, match)
		}
	}
	e.DependantMatches = filteredDependantMatches

	return nil
}

func (s *TournamentStore) handleCourtAssignment(e *CourtEvent) error {
	e.Match = s.matches[e.MatchData.Id]

	if err := e.Next(); err != nil {
		return err
	}

	e.Match.matchData = e.MatchData
	hydrateCourt(e.Match.match, e.MatchData.Court())
	e.AddRealtimeNotification(e.App, "tournament_matches", core.ModelEventTypeUpdate, e.Match, 0)
	return nil
}

func (s *TournamentStore) prepareCourtAssignment(e *CourtEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	e.MatchEvent.ScheduleUpdates = s.collectPlayerMatches(e.MatchEvent.Match.match)
	return nil
}

// Collects the matches that the players of the given match are in
// and maps them by competition
func (s *TournamentStore) collectPlayerMatches(match *got.Match) map[*CompetitionTournament]map[*TournamentMatch]struct{} {
	players := playersInMatch(match)
	matchesToUpdate := make(map[*CompetitionTournament]map[*TournamentMatch]struct{}, 0)
	for competitionId := range s.byCompetitionPlayer {
		tournament := s.tournaments[competitionId]
		if !tournament.Started {
			continue
		}
		tournamentScheduleUpdates := make(map[*TournamentMatch]struct{}, 0)
		for _, p := range players {
			matches := s.byCompetitionPlayer[competitionId][p.Id]
			for _, m := range matches {
				tournamentScheduleUpdates[m] = struct{}{}
			}
		}
		if len(tournamentScheduleUpdates) > 0 {
			matchesToUpdate[tournament] = tournamentScheduleUpdates
		}
	}
	return matchesToUpdate
}

// Allow the replacement of players in a team during the tournament
func (s *TournamentStore) verifyRegistrationUpdate(e *RegistrationEvent) error {
	sizeChanged := len(e.RemovedPlayers) != len(e.AddedPlayers)
	if sizeChanged && s.isRegistrationActive(e.Registration) {
		return errors.New("can not change team size while team is active in a tournament")
	}
	return e.Next()
}

func (s *TournamentStore) verifyUnregistration(e *RegistrationEvent) error {
	if s.isRegistrationActive(e.Registration) {
		return errors.New("can not delete team while it is active in a tournament")
	}
	return e.Next()
}

func (s *TournamentStore) verifyWithdrawal(e *WithdrawEvent) error {
	tournament, ok := s.tournaments[e.Competition.Id]
	if !ok || !tournament.Started || tournament.Ended {
		return errors.New("can not withdraw from competition that is not in progress")
	}
	e.WithdrawalPolicy = tournament
	return e.Next()
}

func (s *TournamentStore) handleStatusChange(e *StatusChangeEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	for _, matchData := range e.ChangedMatchData {
		match := s.matches[matchData.Id]
		match.matchData = matchData
	}
	for _, withdrawal := range e.Withdrawals {
		tPlayer := TournamentPlayer{withdrawal.Registration.Team}
		if e.Withdraw {
			addWithdrawnToMatches(tPlayer, withdrawal.ChangedMatches)
		} else {
			removeWithdrawnFromMatches(tPlayer, withdrawal.ChangedMatches)
		}
		tournament := s.tournaments[withdrawal.Competition.Id]
		s.update(e, tournament)
	}
	return nil
}

func (s *TournamentStore) verifyTieBreakerChange(e *TieBreakerEvent) error {
	tournament := s.tournaments[e.Competition.Id]
	groupKnockout := tournament.Tournament.(*got.GroupKnockout)

	if groupKnockout.KnockOut.MatchList().MatchesStarted() {
		return errors.New("can not change the tie breakers after the knock out phase started")
	}
	e.GroupPhase = groupKnockout.GroupPhase
	return e.Next()
}

func (s *TournamentStore) verifyTieBreakerAdd(e *TieBreakerEvent) error {
	tournament := s.tournaments[e.Competition.Id]
	if tournament == nil || !tournament.Started {
		return errors.New("can not add tie breaker to tournament that is not running")
	}
	_, ok := tournament.Tournament.(*got.GroupKnockout)
	if !ok {
		return errors.New("only group knockout tournaments can have tie breakers added")
	}
	return e.Next()
}

func (s *TournamentStore) handleTieBreakerChange(e *TieBreakerEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	insertTieBreaker(e.Teams, e.GroupPhase)
	tournament := s.tournaments[e.Competition.Id]
	s.update(e, tournament)
	return nil
}

func (s *TournamentStore) handleTieBreakerDelete(e *TieBreakerEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	revokeTieBreaker(e.Teams, e.GroupPhase)
	tournament := s.tournaments[e.Competition.Id]
	s.update(e, tournament)
	return nil
}

func (s *TournamentStore) handleReschedule(e *RescheduleEvent) error {
	e.StartedTournaments = s.listStarted()
	return e.Next()
}

func (s *TournamentStore) handleScheduleStatus(e *ScheduleStatusEvent) error {
	e.MatchData = s.hydratedMatches[e.Match.Id()].matchData
	return e.Next()
}

func (s *TournamentStore) handleRestEnd(e *PlayerRestEvent) error {
	e.Tournament = s.byMatch[e.Match.Id]
	e.MatchEvent.ScheduleUpdates = s.collectPlayerMatches(e.MatchEvent.Match.match)
	return e.Next()
}

func (s *TournamentStore) handleRestSettingsChange(e *MatchRestEvent) error {
	for _, m := range e.Matches {
		scheduleUpdates := s.collectPlayerMatches(m.match)
		for tournament, matches := range scheduleUpdates {
			tournamentUpdates, ok := e.ScheduleUpdates[tournament]
			if ok {
				maps.Copy(tournamentUpdates, matches)
			} else {
				e.ScheduleUpdates[tournament] = matches
			}
		}
	}
	return e.Next()
}

func (s *TournamentStore) verifyCategorizationChange(e *CategorizationEvent) error {
	for _, t := range s.list {
		if t.Started {
			return errors.New("can not edit the categorization after tournaments have been started")
		}
	}
	return e.Next()
}

func (s *TournamentStore) verifyCategoryDelete(e *CategoryDeleteEvent) error {
	competitionStore, _ := store.FindRecordStore[Competition]()
	for _, c := range competitionStore.ListRecords() {
		if !e.Category.IsOfCategory(c) {
			continue
		}
		tournament, ok := s.tournaments[c.Id]
		if ok && tournament.Started {
			return errors.New("can not delete the category of a tournament after it has been started")
		}
	}
	return e.Next()
}

func (s *TournamentStore) verifyModeSettingsUpdate(e *TournamentModeSettingsEvent) error {
	tournament, ok := s.tournaments[e.Competition.Id]
	if ok && tournament.Started {
		return errors.New("can not edit the tournament mode settings of tournament after is has been started")
	}
	return e.Next()
}

func (s *TournamentStore) handleCompetitionDelete(e *CompetitionEvent) error {
	tournament, ok := s.tournaments[e.Competition.Id]
	if ok && tournament.Started {
		return errors.New("can not delete a competition that has a started tournament")
	}

	if err := e.Next(); err != nil {
		return err
	}

	delete(s.tournaments, e.Competition.Id)
	return nil
}

func createMatchData(app core.App, tournament *CompetitionTournament) ([]*MatchData, error) {
	matches := tournament.MatchList().Matches
	matchData := make([]*MatchData, len(matches))
	compId := tournament.Competition.Id
	matchIdGetter := createMatchIdGetter(compId)
	for i := range matches {
		data, err := NewProxy[MatchData](app)
		if err != nil {
			return nil, errors.New("could not create match data proxy")
		}
		data.Id = matchIdGetter(i)
		matchData[i] = data
	}

	return matchData, nil
}

func (s *TournamentStore) hydrate(tournament *CompetitionTournament) error {
	comp := tournament.Competition
	matchData := comp.Matches()

	if len(matchData) == 0 {
		tournament.Started = false
		tournament.Ended = false
		return nil
	}

	tournament.Started = true

	settings := comp.TournamentModeSettings()
	scoreSettings, err := newScoreSettings(settings)
	if err != nil {
		return err
	}
	matches := tournament.MatchList().Matches

	for i := range matches {
		data := matchData[i]
		match := matches[i]

		sets := data.Sets()
		if len(sets) > 0 {
			if err := hydrateScore(match, sets, scoreSettings); err != nil {
				return err
			}
		}

		court := data.Court()
		hydrateCourt(match, court)

		startTime := data.StartTime()
		match.StartTime = startTime.Time()
		endTime := data.EndTime()
		match.EndTime = endTime.Time()

		withdrawn := data.WithdrawnTeams()
		hydrateWithdrawnTeams(match, withdrawn)

		s.hydratedMatches[match.Id()] = s.matches[data.Id]
		s.matches[data.Id].matchData = data
		s.byMatch[data.Id] = tournament
	}

	tournament.Update(nil)

	tournament.Ended = matchesFinished(matches)

	return nil
}

func (s *TournamentStore) dehydrate(tournament *CompetitionTournament) {
	matches := tournament.MatchList().Matches
	for _, m := range matches {
		m.Score = nil
		m.Location = nil
		m.StartTime = time.Time{}
		m.EndTime = time.Time{}
		m.WithdrawnPlayers = nil

		matchData := s.hydratedMatches[m.Id()]
		delete(s.hydratedMatches, m.Id())
		if matchData != nil {
			s.matches[matchData.Id].matchData = nil
			delete(s.byMatch, matchData.Id)
		}
	}
	tournament.Update(nil)
	tournament.Started = false
	tournament.Ended = false
}

func (s *TournamentStore) matchesToMatchData(matches []*got.Match) []*MatchData {
	matchData := make([]*MatchData, len(matches))
	for i, m := range matches {
		matchData[i] = s.hydratedMatches[m.Id()].matchData
	}
	return matchData
}

func (s *TournamentStore) isEditable(matchData *MatchData) bool {
	tournament := s.byMatch[matchData.Id]
	editable := tournament.EditableMatches()
	for _, m := range editable {
		editableMatchData := s.hydratedMatches[m.Id()]
		if editableMatchData.Id == matchData.Id {
			return true
		}
	}
	return false
}

func (s *TournamentStore) isRegistrationActive(reg *Registration) bool {
	team := reg.Team
	comp := reg.Competition
	tournament := s.tournaments[comp.Id]
	if tournament == nil {
		return false
	}

	isInDraw := slices.ContainsFunc(
		comp.Draw(),
		func(t *Team) bool { return t.Id == team.Id },
	)

	isActive := isInDraw && tournament.Started

	return isActive
}

func hydrateScore(match *got.Match, sets []*MatchSet, scoreSettings badminton.ScoreSettings) error {
	score, err := scoreDataToScore(sets, scoreSettings)
	if err != nil {
		return err
	}
	match.Score = score
	return nil
}

func scoreDataToScore(scoreData []*MatchSet, settings badminton.ScoreSettings) (*badminton.Score, error) {
	a := make([]int, 0, settings.WinningSets)
	b := make([]int, 0, settings.WinningSets)
	for _, set := range scoreData {
		a = append(a, set.Team1Points())
		b = append(b, set.Team2Points())
	}

	score, err := badminton.NewScore(a, b, settings)
	return score, err
}

func hydrateCourt(match *got.Match, court *Court) {
	if court == nil {
		match.Location = nil
	} else {
		match.Location = &MatchLocation{court}
	}
}

func hydrateWithdrawnTeams(match *got.Match, withdrawnTeams []*Team) {
	if len(withdrawnTeams) == 0 {
		return
	}

	withdrawn := make([]got.Player, len(withdrawnTeams))
	for i, t := range withdrawnTeams {
		withdrawn[i] = TournamentPlayer{t}
	}

	match.WithdrawnPlayers = withdrawn
}

func addWithdrawnToMatches(team TournamentPlayer, matches []*got.Match) {
	for _, m := range matches {
		m.WithdrawnPlayers = append(m.WithdrawnPlayers, team)
	}
}

func removeWithdrawnFromMatches(team TournamentPlayer, matches []*got.Match) {
	for _, m := range matches {
		updatedWithdrawList := slices.DeleteFunc(
			m.WithdrawnPlayers,
			func(t got.Player) bool { return t.Id() == team.Id() },
		)
		m.WithdrawnPlayers = updatedWithdrawList
	}
}

func insertTieBreaker(tieBreaker []*Team, tournament *got.GroupPhase) {
	tieBreakerRanking := teamsToConstantRanking(tieBreaker)
	for _, group := range tournament.Groups {
		group.FinalRanking.AddTieBreaker(tieBreakerRanking)
	}
	tournament.FinalRanking.AddTieBreaker(tieBreakerRanking)
}

func revokeTieBreaker(tieBreaker []*Team, tournament *got.GroupPhase) {
	tieBreakerRanking := teamsToConstantRanking(tieBreaker)
	for _, group := range tournament.Groups {
		group.FinalRanking.RemoveTieBreaker(tieBreakerRanking)
	}
	tournament.FinalRanking.RemoveTieBreaker(tieBreakerRanking)
}

type TournamentPlayer struct {
	*Team
}

func (p TournamentPlayer) Id() string {
	return p.Team.Id
}

type MatchLocation struct {
	*Court
}

func (l *MatchLocation) Id() string {
	return l.Court.Id
}

var ErrNoDraw error = errors.New("the competition has no draw yet")

func newEntries(comp *Competition) (*got.ConstantRanking, error) {
	draw := comp.Draw()
	if len(draw) == 0 {
		return nil, ErrNoDraw
	}

	entryRanking := teamsToConstantRanking(draw)

	return entryRanking, nil
}

func teamsToConstantRanking(teams []*Team) *got.ConstantRanking {
	tPlayers := make([]got.Player, len(teams))
	for i, t := range teams {
		tPlayers[i] = TournamentPlayer{t}
	}
	ranking := got.NewConstantRanking(tPlayers)
	return ranking
}

func knockoutBuilder(settings *TournamentModeSettings) got.KnockoutBuilder {
	switch settings.KnockOutMode() {
	case Single:
		return got.NewGroupKnockoutSingleElimination
	case Double:
		return got.NewGroupKnockoutDoubleElimination
	case Consolation:
		return got.SingleEliminationWithConsolationBuilder(
			settings.NumConsolationRounds(),
			settings.PlacesToPlayOut(),
		)
	}
	return nil
}

func newScoreSettings(settings *TournamentModeSettings) (badminton.ScoreSettings, error) {
	scoreSettings, err := badminton.NewScoreSettings(
		settings.WinningPoints(),
		settings.WinningSets(),
		settings.MaxPoints(),
		settings.TwoPointMargin(),
	)
	return scoreSettings, err
}

func createMatchIdGetter(competitionId string) func(int) string {
	return func(i int) string {
		return fmt.Sprintf("m-%v-%v", competitionId, i)
	}
}
