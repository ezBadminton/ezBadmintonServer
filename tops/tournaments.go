package tops

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/gotournament/badminton"
	got "github.com/ezBadminton/gotournament/core"
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

func (c *CompetitionTournament) started() bool {
	matches := c.Competition.Matches()
	started := len(matches) > 0
	return started
}

func (c *CompetitionTournament) ended() bool {
	matches := c.Competition.Matches()
	ended := true
	for _, m := range matches {
		sets := m.Sets()
		if len(sets) == 0 {
			ended = false
			break
		}
	}
	return ended
}

var Tournaments *TournamentStore

type TournamentStore struct {
	tournaments map[string]*CompetitionTournament
	list        []*CompetitionTournament
	matchData   map[int]*MatchData
	mu          sync.RWMutex
}

func InitTournaments() error {
	compStore, err := store.FindRecordStore[Competition]()
	if err != nil {
		return err
	}

	Tournaments = &TournamentStore{
		tournaments: make(map[string]*CompetitionTournament),
		list:        make([]*CompetitionTournament, 0),
	}

	err = Tournaments.addTournaments(compStore.RecordList...)
	if err != nil {
		return err
	}

	compStore.RegisterUpdateHandler(CompetitionUpdated)

	return nil
}

func (s *TournamentStore) List() []*CompetitionTournament {
	defer s.mu.RUnlock()
	s.mu.RLock()

	return s.list
}

func (s *TournamentStore) ListRunning() []*CompetitionTournament {
	defer s.mu.RUnlock()
	s.mu.RLock()

	tournaments := make([]*CompetitionTournament, 0, len(s.list))
	for _, t := range s.list {
		if t.Started && !t.Ended {
			tournaments = append(tournaments, t)
		}
	}

	return tournaments
}

func (s *TournamentStore) FindTournament(competitionId string) *CompetitionTournament {
	defer s.mu.RUnlock()
	s.mu.RLock()

	return s.tournaments[competitionId]
}

func (s *TournamentStore) FindMatchData(match *got.Match) *MatchData {
	defer s.mu.RUnlock()
	s.mu.RLock()

	return s.matchData[match.Id()]
}

func (s *TournamentStore) SetTournament(competitionId string, tournament *CompetitionTournament) {
	defer s.mu.Unlock()
	s.mu.Lock()

	s.tournaments[competitionId] = tournament
	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competitionId
	})
	s.list = append(s.list, tournament)

	slices.SortFunc(s.list, compareTournaments)
}

func (s *TournamentStore) RemoveTournament(competitionId string) {
	defer s.mu.Unlock()
	s.mu.Lock()

	tournament := s.tournaments[competitionId]
	if tournament == nil {
		return
	}

	matches := tournament.MatchList().Matches
	for _, m := range matches {
		delete(s.matchData, m.Id())
	}

	delete(s.tournaments, competitionId)
	s.list = slices.DeleteFunc(s.list, func(t *CompetitionTournament) bool {
		return t.Competition.Id == competitionId
	})
}

func (s *TournamentStore) addTournaments(competitions ...*Competition) error {
	defer s.mu.Unlock()
	s.mu.Lock()

	for _, comp := range competitions {
		tournament, err := CreateTournament(comp)
		if errors.Is(err, ErrNoDraw) {
			continue
		}
		if err != nil {
			return err
		}
		hydrate(tournament)
		s.tournaments[comp.Id] = tournament
		s.list = append(s.list, tournament)
	}
	slices.SortFunc(s.list, compareTournaments)
	return nil
}

func CreateTournament(comp *Competition) (*CompetitionTournament, error) {
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
		Competition: comp,
		Tournament:  tournament,
	}

	return compTournament, nil
}

func (s *TournamentStore) HasStarted(competitionId string) (bool, error) {
	defer s.mu.RUnlock()
	s.mu.RLock()

	tournament := s.tournaments[competitionId]
	if tournament == nil {
		return false, errors.New("competition has no draw")
	}

	return tournament.Started, nil
}

func (s *TournamentStore) SetStarted(competitionId string, started bool) error {
	defer s.mu.Unlock()
	s.mu.Lock()

	tournament := s.tournaments[competitionId]
	if tournament == nil {
		return errors.New("competition has no draw")
	}

	if started && tournament.Started {
		return errors.New("competition already running")
	}
	if !started && !tournament.Started {
		return errors.New("competition not running")
	}

	tournament.Started = started
	tournament.Ended = false

	if !started {
		dehydrate(tournament)
	}

	return nil
}

func CompetitionUpdated(_, competition *Competition) {
	data := competition.CustomData()
	newTournament, ok := data["DRAW_CHANGE"]
	if !ok {
		return
	}

	if newTournament != nil {
		t := newTournament.(*CompetitionTournament)
		Tournaments.SetTournament(competition.Id, t)
	} else {
		Tournaments.RemoveTournament(competition.Id)
	}
}

func hydrate(tournament *CompetitionTournament) error {
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
		if err := setScore(match, sets, scoreSettings); err != nil {
			return err
		}

		court := data.Court()
		setCourt(match, court)

		startTime := data.StartTime()
		match.StartTime = startTime.Time()
		endTime := data.EndTime()
		match.EndTime = endTime.Time()

		withdrawn := data.WithdrawnTeams()
		setWithdrawnTeams(match, withdrawn)

		Tournaments.matchData[match.Id()] = data
	}

	tournament.Update(nil)

	tournament.Ended = MatchInfo.MatchesFinished(matches)

	return nil
}

func dehydrate(tournament got.MatchLister) {
	matches := tournament.MatchList().Matches
	for _, m := range matches {
		m.Score = nil
		m.Location = nil
		m.StartTime = time.Time{}
		m.EndTime = time.Time{}
		m.WithdrawnPlayers = nil

		delete(Tournaments.matchData, m.Id())
	}
}

func setScore(match *got.Match, sets []*MatchSet, scoreSettings badminton.ScoreSettings) error {
	if len(sets) == 0 {
		return nil
	}

	a := make([]int, 0, scoreSettings.WinningSets)
	b := make([]int, 0, scoreSettings.WinningSets)
	for _, set := range sets {
		a = append(a, set.Team1Points())
		b = append(b, set.Team2Points())
	}

	score, err := badminton.NewScore(a, b, scoreSettings)
	if err != nil {
		return err
	}

	match.Score = score
	return nil
}

func setCourt(match *got.Match, court *Court) {
	if court == nil {
		return
	}
	match.Location = &MatchLocation{court}
}

func setWithdrawnTeams(match *got.Match, withdrawnTeams []*Team) {
	if len(withdrawnTeams) == 0 {
		return
	}

	withdrawn := make([]got.Player, len(withdrawnTeams))
	for i, t := range withdrawnTeams {
		withdrawn[i] = &TournamentPlayer{t}
	}

	match.WithdrawnPlayers = withdrawn
}

type TournamentPlayer struct {
	*Team
}

func (p *TournamentPlayer) Id() string {
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

	tournamentPlayers := make([]got.Player, len(draw))
	for i, t := range draw {
		tournamentPlayers[i] = &TournamentPlayer{t}
	}

	entryRanking := got.NewConstantRanking(tournamentPlayers)

	return entryRanking, nil
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
