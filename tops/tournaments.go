package tops

import (
	"encoding/json"
	"errors"
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

	json.Marshaler
}

type CompetitionTournament struct {
	BaseTopsRecord
	*Competition
	Tournament
	badminton.ScoreSettings
	Started, Ended bool
}

func (c *CompetitionTournament) ToMap() map[string]any {
	data := map[string]any{
		"competition": c.Competition.Id,
		"tournament":  c.Tournament,
		"started":     c.Started,
		"ended":       c.Ended,
	}
	return c.BaseTopsRecord.ToMap(data)
}

type TournamentStore struct {
	app  core.App
	list []*CompetitionTournament
	// Competition id -> tournament
	tournaments map[string]*CompetitionTournament
	// Tournament match id -> match data
	matchData map[int]*MatchData
	// match data id -> tournament match
	matches map[string]*got.Match
	// match data id -> tournament
	byMatch map[string]*CompetitionTournament

	// Before tournament start
	onStart *hook.Hook[*PlanEvent]
	// After match data created. After e.Next() the match data has been persisted and the tournament hydrated
	onAfterStart *hook.Hook[*PlanEvent]

	// Before tournament stop
	onStop *hook.Hook[*PlanEvent]
	// After match data to delete is set. After e.Next() the match data deletion has been persisted. Dehydration happens after the hook
	onAfterStop *hook.Hook[*PlanEvent]

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
) *TournamentStore {
	compStore, _ := store.FindRecordStore[Competition]()

	s := &TournamentStore{
		app:          app,
		tournaments:  make(map[string]*CompetitionTournament),
		list:         make([]*CompetitionTournament, 0),
		matchData:    make(map[int]*MatchData),
		matches:      make(map[string]*got.Match),
		byMatch:      make(map[string]*CompetitionTournament),
		onStart:      &hook.Hook[*PlanEvent]{},
		onAfterStart: &hook.Hook[*PlanEvent]{},
		onStop:       &hook.Hook[*PlanEvent]{},
		onAfterStop:  &hook.Hook[*PlanEvent]{},
		onUpdate:     &hook.Hook[*PlanEvent]{},
	}

	err := s.addTournaments(compStore.RecordList...)
	if err != nil {
		panic("unable to initialize tournament store")
	}

	drawManager.onDraw.BindFunc(s.verifyDrawChange)
	drawManager.onAfterDraw.BindFunc(s.handleDraw)
	drawManager.onDrawDelete.BindFunc(s.verifyDrawDeletion)
	drawManager.onAfterDrawDelete.BindFunc(s.handleDrawDeletion)
	drawManager.onDrawSwap.BindFunc(s.verifyDrawChange)
	drawManager.onAfterDrawSwap.BindFunc(s.handleDraw)
	drawManager.onSetSeeds.BindFunc(s.verifyDrawChange)

	courtStore.onAfterCourtAssign.BindFunc(s.handleCourtAssignment)
	courtStore.onAfterCourtUnassign.BindFunc(s.handleCourtAssignment)

	matchManager.onAfterStart.BindFunc(s.handleMatchStart)
	matchManager.onAfterCancel.BindFunc(s.handleMatchCancel)
	matchManager.onScoreSet.BindFunc(s.verifyScoreSet)
	matchManager.onAfterScoreSet.BindFunc(s.handleScoreSet)
	matchManager.onReset.BindFunc(s.verifyMatchReset)
	matchManager.onAfterReset.BindFunc(s.handleMatchReset)

	registrationStore.onAfterUpdate.BindFunc(s.verifyRegistrationUpdate)
	registrationStore.onDelete.BindFunc(s.verifyUnregistration)

	withdrawalManager.onWithdraw.BindFunc(s.verifyWithdrawal)
	withdrawalManager.onAfterStatusChange.BindFunc(s.handleStatusChange)

	tieBreakerManager.onAdd.BindFunc(s.verifyTieBreakerAdd)
	tieBreakerManager.onAdd.BindFunc(s.verifyTieBreakerChange)
	tieBreakerManager.onAfterAdd.BindFunc(s.handleTieBreakerChange)
	tieBreakerManager.onUpdate.BindFunc(s.verifyTieBreakerChange)
	tieBreakerManager.onAfterUpdate.BindFunc(s.handleTieBreakerChange)
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

func (s *TournamentStore) setTournament(competition *Competition, tournament *CompetitionTournament) {
	_, isUpdate := s.tournaments[competition.Id]

	s.tournaments[competition.Id] = tournament

	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competition.Id
	})
	s.list = append(s.list, tournament)
	slices.SortFunc(s.list, compareTournaments)

	var realtimeEventType string
	if isUpdate {
		realtimeEventType = core.ModelEventTypeUpdate
	} else {
		realtimeEventType = core.ModelEventTypeCreate
	}

	go realtimeNotify(s.app, "tournamentplans", realtimeEventType, tournament)
}

func (s *TournamentStore) update(tournament *CompetitionTournament) {
	event := newPlanEvent(s.app, tournament.Competition, tournament)
	s.onUpdate.Trigger(event, func(e *PlanEvent) error {
		e.Tournament.Update(nil)
		go realtimeNotify(s.app, "tournamentplans", core.ModelEventTypeUpdate, tournament)
		return e.Next()
	})
}

func (s *TournamentStore) removeTournament(competition *Competition) {
	tournament := s.tournaments[competition.Id]
	matches := tournament.MatchList().Matches
	for _, m := range matches {
		delete(s.matchData, m.Id())
		matchData := s.matchData[m.Id()]
		if matchData != nil {
			delete(s.matches, matchData.Id)
			delete(s.byMatch, matchData.Id)
		}
	}

	delete(s.tournaments, competition.Id)
	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competition.Id
	})

	go realtimeNotify(s.app, "tournamentplans", core.ModelEventTypeDelete, tournament)
}

func (s *TournamentStore) addTournaments(competitions ...*Competition) error {
	for _, comp := range competitions {
		tournament, err := s.createTournament(comp)
		if tournament == nil && err == nil {
			continue
		}
		if err != nil {
			return err
		}
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
	if errors.Is(err, ErrNoDraw) {
		return nil, nil
	} else if err != nil {
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
	return s.onStart.Trigger(event, s.startHandler)
}

func (s *TournamentStore) startHandler(e *PlanEvent) error {
	matchData, err := createMatchData(e.App, e.Tournament)
	if err != nil {
		return err
	}
	e.MatchData = matchData

	err = s.onAfterStart.Trigger(e,
		(*PlanEvent).saveStartedPlan,
		s.hydrateHandler,
	)
	return e.Next()
}

func (s *TournamentStore) hydrateHandler(e *PlanEvent) error {
	s.hydrate(e.Tournament)
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
	return s.onStop.Trigger(event, s.stopHandler)
}

func (s *TournamentStore) stopHandler(e *PlanEvent) error {
	e.MatchData = e.Competition.Matches()

	err := s.onAfterStop.Trigger(e, (*PlanEvent).saveStoppedPlan)
	if err != nil {
		return err
	}
	s.dehydrate(e.Tournament)
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
	tournament, err := s.createTournament(e.Competition)
	if err != nil {
		return err
	}
	if err := e.Next(); err != nil {
		return err
	}
	s.setTournament(e.Competition, tournament)
	return nil
}

func (s *TournamentStore) verifyDrawDeletion(e *CompetitionEvent) error {
	tournament := s.tournaments[e.Competition.Id]
	if tournament == nil {
		return errors.New("this competition has no draw. Can not delete draw")
	}
	if tournament.Started {
		return errors.New("can not delete draw of running tournament")
	}
	return e.Next()
}

func (s *TournamentStore) handleDrawDeletion(e *CompetitionEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	s.removeTournament(e.Competition)
	return nil
}

func (s *TournamentStore) handleMatchStart(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	match := s.matches[e.MatchData.Id]
	match.StartTime = e.MatchData.StartTime().Time()

	tournament := s.byMatch[e.MatchData.Id]
	tournament.UpdateEditableMatches()
	return nil
}

func (s *TournamentStore) handleMatchCancel(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	match := s.matches[e.MatchData.Id]
	match.StartTime = time.Time{}

	tournament := s.byMatch[e.MatchData.Id]
	tournament.UpdateEditableMatches()
	return nil
}

func (s *TournamentStore) verifyScoreSet(e *ScoreEvent) error {
	if !e.MatchData.EndTime().IsZero() && !s.isEditable(e.MatchData) {
		return errors.New("the match is not in an editable state")
	}
	e.Match = s.matches[e.MatchData.Id]
	return e.Next()
}

func (s *TournamentStore) handleScoreSet(e *ScoreEvent) error {
	tournament := s.byMatch[e.MatchData.Id]
	score, err := scoreDataToScore(e.ScoreData, tournament.ScoreSettings)
	if err != nil {
		return err
	}
	matchEnding := e.MatchData.EndTime().IsZero()

	if err := e.Next(); err != nil {
		return err
	}

	match := s.matches[e.MatchData.Id]
	match.Score = score
	if matchEnding {
		match.EndTime = e.MatchData.EndTime().Time()
		tournament.Ended = matchesFinished(tournament.MatchList().Matches)
	}
	s.update(tournament)
	return nil
}

func (s *TournamentStore) verifyMatchReset(e *ScoreEvent) error {
	if !s.isEditable(e.MatchData) {
		return errors.New("the match is not in an editable state")
	}
	return e.Next()
}

func (s *TournamentStore) handleMatchReset(e *ScoreEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	match := s.matches[e.MatchData.Id]
	match.Score = nil
	match.StartTime = time.Time{}
	match.EndTime = time.Time{}
	if e.MatchData.Court() == nil {
		match.Location = nil
	}

	tournament := s.byMatch[e.MatchData.Id]
	tournament.Ended = false
	s.update(tournament)
	return nil
}

func (s *TournamentStore) handleCourtAssignment(e *CourtEvent) error {
	e.Match = s.matches[e.MatchData.Id]

	if err := e.Next(); err != nil {
		return err
	}

	hydrateCourt(e.Match, e.MatchData.Court())
	return nil
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

	for _, withdrawal := range e.Withdrawals {
		tPlayer := TournamentPlayer{withdrawal.Registration.Team}
		if e.Withdraw {
			addWithdrawnToMatches(tPlayer, withdrawal.ChangedMatches)
		} else {
			removeWithdrawnFromMatches(tPlayer, withdrawal.ChangedMatches)
		}
		tournament := s.tournaments[withdrawal.Competition.Id]
		s.update(tournament)
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
	s.update(tournament)
	return nil
}

func (s *TournamentStore) handleTieBreakerDelete(e *TieBreakerEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	revokeTieBreaker(e.Teams, e.GroupPhase)
	tournament := s.tournaments[e.Competition.Id]
	s.update(tournament)
	return nil
}

func (s *TournamentStore) handleReschedule(e *RescheduleEvent) error {
	e.StartedTournaments = s.listStarted()
	return e.Next()
}

func (s *TournamentStore) handleScheduleStatus(e *ScheduleStatusEvent) error {
	e.MatchData = s.matchData[e.Match.Id()]
	return e.Next()
}

func (s *TournamentStore) handleRestEnd(e *PlayerRestEvent) error {
	e.Tournament = s.byMatch[e.MatchData.Id]
	return e.Next()
}

func (s *TournamentStore) handleRestSettingsChange(e *MatchRestEvent) error {
	tournamentSet := make(map[*CompetitionTournament]any)
	for _, m := range e.MatchData {
		t := s.byMatch[m.Id]
		tournamentSet[t] = struct{}{}
	}
	e.Tournaments = make([]*CompetitionTournament, 0, len(tournamentSet))
	for t := range tournamentSet {
		e.Tournaments = append(e.Tournaments, t)
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

func createMatchData(app core.App, tournament got.MatchLister) ([]*MatchData, error) {
	matches := tournament.MatchList().Matches
	matchData := make([]*MatchData, len(matches))
	for i := range matches {
		data, err := NewProxy[MatchData](app)
		if err != nil {
			return nil, errors.New("could not create match data proxy")
		}
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

		s.matchData[match.Id()] = data
		s.matches[data.Id] = match
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

		delete(s.matchData, m.Id())
		matchData := s.matchData[m.Id()]
		if matchData != nil {
			delete(s.matches, matchData.Id)
			delete(s.byMatch, matchData.Id)
		}
	}
	tournament.Started = false
	tournament.Ended = false
}

func (s *TournamentStore) matchesToMatchData(matches []*got.Match) []*MatchData {
	matchData := make([]*MatchData, len(matches))
	for i, m := range matches {
		matchData[i] = s.matchData[m.Id()]
	}
	return matchData
}

func (s *TournamentStore) isEditable(matchData *MatchData) bool {
	tournament := s.byMatch[matchData.Id]
	editable := tournament.EditableMatches()
	for _, m := range editable {
		editableMatchData := s.matchData[m.Id()]
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
