package collection

import "github.com/ezBadminton/ezBadmintonServer/generated"

type Enum int

const (
	TournamentOrganizers Enum = iota
	AgeGroups
	Clubs
	Competitions
	Courts
	Gymnasiums
	MatchData
	MatchSets
	Players
	PlayingLevels
	Teams
	TieBreakers
	TournamentModeSettings
	Tournaments
)

var Names = map[Enum]string{
	TournamentOrganizers:   "tournament_organizer",
	AgeGroups:              "age_groups",
	Clubs:                  "clubs",
	Competitions:           "competitions",
	Courts:                 "courts",
	Gymnasiums:             "gymnasiums",
	MatchData:              "match_data",
	MatchSets:              "match_sets",
	Players:                "players",
	PlayingLevels:          "playing_levels",
	Teams:                  "teams",
	TieBreakers:            "tie_breakers",
	TournamentModeSettings: "tournament_mode_settings",
	Tournaments:            "tournaments",
}

// Returns the collection that a proxy or slice slice of proxies belongs to
func FromProxy(s any) Enum {
	switch s.(type) {
	case *generated.TournamentOrganizer, []*generated.TournamentOrganizer, RecordList[*generated.TournamentOrganizer]:
		return TournamentOrganizers
	case *generated.AgeGroup, []*generated.AgeGroup, RecordList[*generated.AgeGroup]:
		return AgeGroups
	case *generated.Club, []*generated.Club, RecordList[*generated.Club]:
		return Clubs
	case *generated.Competition, []*generated.Competition, RecordList[*generated.Competition]:
		return Competitions
	case *generated.Court, []*generated.Court, RecordList[*generated.Court]:
		return Courts
	case *generated.Gymnasium, []*generated.Gymnasium, RecordList[*generated.Gymnasium]:
		return Gymnasiums
	case *generated.MatchData, []*generated.MatchData, RecordList[*generated.MatchData]:
		return MatchData
	case *generated.MatchSet, []*generated.MatchSet, RecordList[*generated.MatchSet]:
		return MatchSets
	case *generated.Player, []*generated.Player, RecordList[*generated.Player]:
		return Players
	case *generated.PlayingLevel, []*generated.PlayingLevel, RecordList[*generated.PlayingLevel]:
		return PlayingLevels
	case *generated.Team, []*generated.Team, RecordList[*generated.Team]:
		return Teams
	case *generated.TieBreaker, []*generated.TieBreaker, RecordList[*generated.TieBreaker]:
		return TieBreakers
	case *generated.TournamentModeSettings, []*generated.TournamentModeSettings, RecordList[*generated.TournamentModeSettings]:
		return TournamentModeSettings
	case *generated.Tournament, []*generated.Tournament, RecordList[*generated.Tournament]:
		return Tournaments
	}

	panic("Unknown proxy type")
}
