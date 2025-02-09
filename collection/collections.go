package collection

import (
	"errors"

	"github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

// Collection names
const (
	TournamentOrganizers   = "tournament_organizer"
	AgeGroups              = "age_groups"
	Clubs                  = "clubs"
	Competitions           = "competitions"
	Courts                 = "courts"
	Gymnasiums             = "gymnasiums"
	MatchData              = "match_data"
	MatchSets              = "match_sets"
	Players                = "players"
	PlayingLevels          = "playing_levels"
	Teams                  = "teams"
	TieBreakers            = "tie_breakers"
	TournamentModeSettings = "tournament_mode_settings"
	Tournaments            = "tournaments"
)

type relationField struct {
	fieldName string
	isMulti   bool
}

// collection name -> related collection name -> relation fields
var Relations = map[string]map[string][]relationField{
	Competitions: {
		AgeGroups:     {{"ageGroup", false}},
		PlayingLevels: {{"playingLevel", false}},
		Teams: {
			{"registrations", true},
			{"seeds", true},
			{"draw", true},
		},
		TournamentModeSettings: {{"tournamentModeSettings", false}},
		MatchData:              {{"matches", true}},
		TieBreakers:            {{"tieBreakers", true}},
	},
	Courts: {
		Gymnasiums: {{"gymnasium", false}},
	},
	MatchData: {
		MatchSets: {{"sets", true}},
		Courts:    {{"court", false}},
		Teams:     {{"withdrawnTeams", true}},
	},
	Players: {
		Clubs: {{"club", false}},
	},
	Teams: {
		Players: {{"players", true}},
	},
	TieBreakers: {
		Teams: {{"tieBreakerRanking", true}},
	},
}

// Returns the collection name that a proxy or slice of proxies belongs to
func CollectionNameFromProxy(s any) string {
	switch s.(type) {
	case *generated.TournamentOrganizer, []*generated.TournamentOrganizer:
		return TournamentOrganizers
	case *generated.AgeGroup, []*generated.AgeGroup:
		return AgeGroups
	case *generated.Club, []*generated.Club:
		return Clubs
	case *generated.Competition, []*generated.Competition:
		return Competitions
	case *generated.Court, []*generated.Court:
		return Courts
	case *generated.Gymnasium, []*generated.Gymnasium:
		return Gymnasiums
	case *generated.MatchData, []*generated.MatchData:
		return MatchData
	case *generated.MatchSet, []*generated.MatchSet:
		return MatchSets
	case *generated.Player, []*generated.Player:
		return Players
	case *generated.PlayingLevel, []*generated.PlayingLevel:
		return PlayingLevels
	case *generated.Team, []*generated.Team:
		return Teams
	case *generated.TieBreaker, []*generated.TieBreaker:
		return TieBreakers
	case *generated.TournamentModeSettings, []*generated.TournamentModeSettings:
		return TournamentModeSettings
	case *generated.Tournament, []*generated.Tournament:
		return Tournaments
	}

	panic("Unknown proxy type")
}

// Creates a new proxy (without underlying record) for the given
// collectionName
func NewProxyFromCollectionName(collectionName string) core.RecordProxy {
	switch collectionName {
	case TournamentOrganizers:
		return &generated.TournamentOrganizer{}
	case AgeGroups:
		return &generated.AgeGroup{}
	case Clubs:
		return &generated.Club{}
	case Competitions:
		return &generated.Competition{}
	case Courts:
		return &generated.Court{}
	case Gymnasiums:
		return &generated.Gymnasium{}
	case MatchData:
		return &generated.MatchData{}
	case MatchSets:
		return &generated.MatchSet{}
	case Players:
		return &generated.Player{}
	case PlayingLevels:
		return &generated.PlayingLevel{}
	case Teams:
		return &generated.Team{}
	case TieBreakers:
		return &generated.TieBreaker{}
	case TournamentModeSettings:
		return &generated.TournamentModeSettings{}
	case Tournaments:
		return &generated.Tournament{}
	}
	return nil
}

// Creates a new record and wraps it in a new proxy
func NewProxy[P core.RecordProxy](app core.App) (P, error) {
	var p P
	collectionName := CollectionNameFromProxy(p)
	p = NewProxyFromCollectionName(collectionName).(P)

	collection, err := app.FindCachedCollectionByNameOrId(collectionName)
	if err != nil {
		return p, err
	}

	record := core.NewRecord(collection)
	p.SetProxyRecord(record)
	return p, nil
}

// Wraps a record in a newly created proxy
func Wrap[P core.RecordProxy](record *core.Record) (P, error) {
	var p P
	collectionName := record.Collection().Name
	p, ok := NewProxyFromCollectionName(collectionName).(P)
	if !ok {
		return p, errors.New("the generic proxy type is not from the collection of the given record.")
	}
	p.SetProxyRecord(record)
	return p, nil
}

// Finds the stored proxy of a record
func FindProxy[P core.RecordProxy](record *core.Record) (P, error) {
	var p P
	collectionName := record.Collection().Name
	store, err := FindRecordStore(collectionName)
	if err != nil {
		return p, err
	}

	found, ok := store.FindRecord(record.Id)
	if !ok {
		return p, errors.New("the record has no stored proxy")
	}

	return found.(P), nil
}

func fetchCollection[P core.RecordProxy](app core.App) ([]P, error) {
	records := make([]P, 0)
	collectionName := CollectionNameFromProxy(records)
	query := app.RecordQuery(collectionName)

	if err := query.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}
