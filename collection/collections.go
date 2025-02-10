package collection

import (
	"errors"

	g "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

type Proxy interface {
	g.TournamentOrganizer |
		g.AgeGroup |
		g.Club |
		g.Competition |
		g.Court |
		g.Gymnasium |
		g.MatchData |
		g.MatchSet |
		g.Player |
		g.PlayingLevel |
		g.Team |
		g.TieBreaker |
		g.TournamentModeSettings |
		g.Tournament
}

type ProxyP[P Proxy] interface {
	*P
	core.RecordProxy
	CollectionName() string
}

type ProxyS[P Proxy] interface {
	*P | []*P
}

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

// Creates a new record and wraps it in a new proxy
func NewProxy[P Proxy, PP ProxyP[P]](app core.App) (PP, error) {
	var p PP = &P{}

	collectionName := p.CollectionName()
	collection, err := app.FindCachedCollectionByNameOrId(collectionName)
	if err != nil {
		return p, err
	}

	record := core.NewRecord(collection)
	p.SetProxyRecord(record)
	return p, nil
}

// Wraps a record in a newly created proxy
func WrapRecord[PP ProxyP[P], P Proxy](record *core.Record) (PP, error) {
	collectionName := record.Collection().Name
	proxyCollectionName := PP.CollectionName(nil)
	if collectionName != proxyCollectionName {
		return nil, errors.New("the generic proxy type is not of the same collection as the given record")
	}
	var p PP = &P{}
	p.SetProxyRecord(record)
	return p, nil
}

// Finds the stored proxy of a record
func FindProxy[P Proxy, PP ProxyP[P]](record *core.Record) (PP, error) {
	collectionName := record.Collection().Name
	proxyCollectionName := PP.CollectionName(nil)
	if collectionName != proxyCollectionName {
		return nil, errors.New("the generic proxy type is not of the same collection as the given record")
	}
	store, err := FindRecordStore[PP]()
	if err != nil {
		return nil, err
	}

	found, ok := store.FindProxy(record.Id)
	if !ok {
		return nil, errors.New("the record has no stored proxy")
	}

	return found, nil
}

func fetchCollection[PP ProxyP[P], P Proxy](app core.App) ([]PP, error) {
	records := make([]PP, 0)
	collectionName := PP.CollectionName(nil)
	query := app.RecordQuery(collectionName)

	if err := query.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}
