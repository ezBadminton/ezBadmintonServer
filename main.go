package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
)

// PB schema field names
const (
	useAgeGroupsName           = "useAgeGroups"           // From Tournament model
	usePlayingLevelsName       = "usePlayingLevels"       // From Tournament model
	ageGroupsName              = "age_groups"             // AgeGroup collection name
	playingLevelsName          = "playing_levels"         // PlayingLevel collection name
	competitionsName           = "competitions"           // Competition collection name
	ageGroupName               = "ageGroup"               // From Competition model
	playingLevelName           = "playingLevel"           // From Competition model
	registrationsName          = "registrations"          // From Competition model
	drawName                   = "draw"                   // From Competition model
	seedsName                  = "seeds"                  // From Competition model
	tournamentModeSettingsName = "tournamentModeSettings" // From Competition model
	genderCategoryName         = "genderCategory"         // From Competition model
	teamSizeName               = "teamSize"               // From Competition model
	competitionMatchesName     = "matches"                // From Competition model
	competitionTieBreakersName = "tieBreakers"            // From Competition model
	playersName                = "players"                // Player collection name
	tournamentsName            = "tournaments"            // Tournament collection name
	teamsName                  = "teams"                  // Team collection name
	teamPlayersName            = "players"                // From Team model
	matchesName                = "matches"                // Match collection name
	matchDataName              = "match_data"             // MatchData collection name
	matchDataCourtName         = "court"                  // From MatchData model
	matchDataSetsName          = "sets"                   // From MatchData model
	courtsName                 = "courts"                 // Court collection name
	courtGymnasiumName         = "gymnasium"              // From Court model
	matchSetsName              = "match_sets"             // MatchSet collection name
	matchEndTimeName           = "endTime"                // From MatchData model
	matchSetPoints1Name        = "team1Points"            // From MatchSet model
	matchSetPoints2Name        = "team2Points"            // From MatchSet model
	gymnasiumsName             = "gymnasiums"             // Gymnasium collection name
)

func main() {
	app := pocketbase.New()

	RegisterHooks(app)
	RegisterRoutes(app)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
