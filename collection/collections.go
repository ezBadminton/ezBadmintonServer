package collection

import (
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
		Courts:    {{"courts", false}},
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

// Returns the collection name that a proxy or slice slice of proxies belongs to
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

func NewProxy(collectionName string) core.RecordProxy {
	switch collectionName {
	case "tournament_organizer":
		return &generated.TournamentOrganizer{}
	case "age_groups":
		return &generated.AgeGroup{}
	case "clubs":
		return &generated.Club{}
	case "competitions":
		return &generated.Competition{}
	case "courts":
		return &generated.Court{}
	case "gymnasiums":
		return &generated.Gymnasium{}
	case "match_data":
		return &generated.MatchData{}
	case "match_sets":
		return &generated.MatchSet{}
	case "players":
		return &generated.Player{}
	case "playing_levels":
		return &generated.PlayingLevel{}
	case "teams":
		return &generated.Team{}
	case "tie_breakers":
		return &generated.TieBreaker{}
	case "tournament_mode_settings":
		return &generated.TournamentModeSettings{}
	case "tournaments":
		return &generated.Tournament{}
	}
	return nil
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
