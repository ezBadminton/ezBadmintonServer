package schema_names

// PB schema collection and field names

var Collections = struct {
	TournamentOrganizer    string
	AgeGroups              string
	Clubs                  string
	Competitions           string
	Courts                 string
	Gymnasiums             string
	MatchData              string
	MatchSets              string
	Players                string
	PlayingLevels          string
	Teams                  string
	TieBreakers            string
	TournamentModeSettings string
	Tournaments            string
}{
	TournamentOrganizer:    "tournament_organizer",
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

var Fields = struct {
	AgeGroups    struct{}
	Clubs        struct{}
	Competitions struct {
		AgeGroup               string
		PlayingLevel           string
		Registrations          string
		Draw                   string
		Seeds                  string
		TournamentModeSettings string
		GenderCategory         string
		TeamSize               string
		Matches                string
		TieBreakers            string
	}
	Courts     struct{ Gymnasium string }
	Gymnasiums struct{}
	MatchData  struct {
		Court   string
		Sets    string
		EndTime string
	}
	MatchSets struct {
		Team1Points string
		Team2Points string
	}
	Players       struct{}
	PlayingLevels struct{}
	Teams         struct {
		Players string
	}
	tieBreakers            struct{}
	tournamentModeSettings struct{}
	Tournaments            struct {
		Title                 string
		UseAgeGroups          string
		UsePlayingLevels      string
		DontReprintGameSheets string
		PrintQrCodes          string
		PlayerRestTime        string
		QueueMode             string
	}
}{
	Competitions: struct {
		AgeGroup               string
		PlayingLevel           string
		Registrations          string
		Draw                   string
		Seeds                  string
		TournamentModeSettings string
		GenderCategory         string
		TeamSize               string
		Matches                string
		TieBreakers            string
	}{
		AgeGroup:               "ageGroup",
		PlayingLevel:           "playingLevel",
		Registrations:          "registrations",
		Draw:                   "draw",
		Seeds:                  "seeds",
		TournamentModeSettings: "tournamentModeSettings",
		GenderCategory:         "genderCategory",
		TeamSize:               "teamSize",
		Matches:                "matches",
		TieBreakers:            "tieBreakers",
	},
	Courts: struct {
		Gymnasium string
	}{
		Gymnasium: "gymnasium",
	},
	MatchData: struct {
		Court   string
		Sets    string
		EndTime string
	}{
		Court:   "court",
		Sets:    "sets",
		EndTime: "endTime",
	},
	MatchSets: struct {
		Team1Points string
		Team2Points string
	}{
		Team1Points: "team1Points",
		Team2Points: "team2Points",
	},
	Teams: struct {
		Players string
	}{
		Players: "players",
	},
	Tournaments: struct {
		Title                 string
		UseAgeGroups          string
		UsePlayingLevels      string
		DontReprintGameSheets string
		PrintQrCodes          string
		PlayerRestTime        string
		QueueMode             string
	}{
		Title:                 "title",
		UseAgeGroups:          "useAgeGroups",
		UsePlayingLevels:      "usePlayingLevels",
		DontReprintGameSheets: "dontReprintGameSheets",
		PrintQrCodes:          "printQrCodes",
		PlayerRestTime:        "playerRestTime",
		QueueMode:             "queueMode",
	},
}
