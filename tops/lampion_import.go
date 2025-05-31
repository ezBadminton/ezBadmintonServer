package tops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

type LampionImporter struct {
	App core.App
	Url string
}

func newLampionImporter(app core.App) *LampionImporter {
	return &LampionImporter{
		App: app,
		Url: "https://www.tgcamberg1848.de/angebot/ballsport/badminton/lampionturnier/getRecords",
	}
}

func (l *LampionImporter) importLampionTournament() error {
	if err := l.setUpCompetitions(); err != nil {
		return err
	}
	if err := l.importEntries(); err != nil {
		return err
	}
	return nil
}

func (l *LampionImporter) setUpCompetitions() error {
	eventStore, _ := store.FindRecordStore[TournamentEvent]()
	event := Clone(eventStore.RecordList[0])

	if !event.UsePlayingLevels() {
		event.SetUsePlayingLevels(true)
		if err := l.App.Save(event); err != nil {
			return err
		}
	}

	levelNames := []string{
		"Gruppe T",
		"Gruppe G",
		"Gruppe C",
		"Gruppe B",
	}

	womensSingles, _ := NewProxy[Competition](l.App)
	womensSingles.SetGenderCategory(Female)
	womensSingles.SetTeamSize(1)
	mensSingles, _ := NewProxy[Competition](l.App)
	mensSingles.SetGenderCategory(Male)
	mensSingles.SetTeamSize(1)
	womensDoubles, _ := NewProxy[Competition](l.App)
	womensDoubles.SetGenderCategory(Female)
	womensDoubles.SetTeamSize(2)
	mensDoubles, _ := NewProxy[Competition](l.App)
	mensDoubles.SetGenderCategory(Male)
	mensDoubles.SetTeamSize(2)
	mixedDoubles, _ := NewProxy[Competition](l.App)
	mixedDoubles.SetGenderCategory(Mixed)
	mixedDoubles.SetTeamSize(2)
	competitions := []*Competition{
		womensSingles,
		mensSingles,
		womensDoubles,
		mensDoubles,
		mixedDoubles,
	}

	for i, name := range levelNames {
		level, _ := NewProxy[PlayingLevel](l.App)
		level.SetIndex(i)
		level.SetName(name)
		if err := l.App.Save(level); err != nil {
			return err
		}

		for _, competition := range competitions {
			competition := Clone(competition)
			competition.SetPlayingLevel(level)
			if err := l.App.Save(competition); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *LampionImporter) importEntries() error {
	res, err := http.Get(l.Url)
	if err != nil {
		return err
	}
	buf := bytes.Buffer{}
	buf.ReadFrom(res.Body)
	responseJson := map[string]any{}
	json.Unmarshal(buf.Bytes(), &responseJson)

	rawEntries := responseJson["Meldungen"].([]any)
	clubs, err := l.createClubs(rawEntries)
	if err != nil {
		return err
	}

	players, err := l.createPlayers(rawEntries, clubs)
	if err != nil {
		return err
	}

	err = l.createAndRegisterTeams(rawEntries, players)
	return err
}

func (l *LampionImporter) createClubs(rawEntries []any) (map[string]*Club, error) {
	nameSet := map[string]any{}

	for _, entry := range rawEntries {
		entry := entry.(map[string]any)

		club1 := entry["Verein1"].(string)
		club2 := entry["Verein2"].(string)
		if club1 != "" {
			nameSet[club1] = struct{}{}
		}
		if club2 != "" {
			nameSet[club2] = struct{}{}
		}
	}

	clubs := make(map[string]*Club)
	for clubName := range nameSet {
		club, _ := NewProxy[Club](l.App)
		club.SetName(clubName)
		if err := l.App.Save(club); err != nil {
			return nil, err
		}
		clubs[clubName] = club
	}

	return clubs, nil
}

func (l *LampionImporter) createPlayers(rawEntries []any, clubs map[string]*Club) (map[string]*Player, error) {
	nameSet := map[string]any{}

	players := make(map[string]*Player)

	for _, entry := range rawEntries {
		entry := entry.(map[string]any)
		for _, playerIndex := range []string{"1", "2"} {
			rawName := entry[fmt.Sprintf("Spieler%v", playerIndex)].(string)
			rawClubName := entry[fmt.Sprintf("Verein%v", playerIndex)].(string)
			firstName, lastName, clubName := parsePlayer(rawName, rawClubName)
			if firstName == "" {
				continue
			}
			_, ok := nameSet[firstName+lastName+clubName]
			if ok {
				continue
			}
			nameSet[firstName+lastName+clubName] = struct{}{}

			club, _ := clubs[clubName]
			player, _ := NewProxy[Player](l.App)
			player.SetStatus(NotAttending)
			player.SetFirstName(firstName)
			player.SetLastName(lastName)
			player.SetClub(club)

			if err := l.App.Save(player); err != nil {
				return nil, err
			}

			players[firstName+lastName+clubName] = player
		}
	}
	return players, nil
}

func (l *LampionImporter) createAndRegisterTeams(rawEntries []any, players map[string]*Player) error {
	competitions := map[string]map[string]*Competition{}

	playingLevelStore, _ := store.FindRecordStore[PlayingLevel]()
	for _, level := range playingLevelStore.RecordList {
		competitions[level.Name()] = map[string]*Competition{}
	}

	competitionStore, _ := store.FindRecordStore[Competition]()
	for _, competition := range competitionStore.RecordList {
		switch {
		case competition.TeamSize() == 1 && competition.GenderCategory() == Female:
			competitions[competition.PlayingLevel().Name()]["DE"] = competition
		case competition.TeamSize() == 1 && competition.GenderCategory() == Male:
			competitions[competition.PlayingLevel().Name()]["HE"] = competition
		case competition.TeamSize() == 2 && competition.GenderCategory() == Female:
			competitions[competition.PlayingLevel().Name()]["DD"] = competition
		case competition.TeamSize() == 2 && competition.GenderCategory() == Male:
			competitions[competition.PlayingLevel().Name()]["HD"] = competition
		case competition.TeamSize() == 2 && competition.GenderCategory() == Mixed:
			competitions[competition.PlayingLevel().Name()]["MD"] = competition
		}
	}

	for _, entry := range rawEntries {
		entry := entry.(map[string]any)
		team, _ := NewProxy[Team](l.App)
		teamMembers := make([]*Player, 0)
		for _, playerIndex := range []string{"1", "2"} {
			rawName := entry[fmt.Sprintf("Spieler%v", playerIndex)].(string)
			rawClubName := entry[fmt.Sprintf("Verein%v", playerIndex)].(string)
			firstName, lastName, clubName := parsePlayer(rawName, rawClubName)
			if firstName == "" {
				continue
			}

			player := players[firstName+lastName+clubName]
			teamMembers = append(teamMembers, player)
		}
		team.SetPlayers(teamMembers)

		playingLevel := entry["Spielklasse"].(string)
		discipline := entry["Disziplin"].(string)[:2]
		competition := competitions[playingLevel][discipline]

		if err := tops.registrationStore.registerTeam(l.App, team, competition); err != nil {
			if strings.Contains(err.Error(), "already registered") {
				fmt.Printf("The player %v %v is registered multiple times.\n", teamMembers[0].FirstName(), teamMembers[0].LastName())
			} else {
				return err
			}
		}
	}
	return nil
}

// Returns first name, last name, club name
func parsePlayer(rawName, rawClubName string) (string, string, string) {
	if rawName == "" || rawName == "Freimeldung" {
		return "", "", ""
	}
	var firstName, lastName string
	var split []string
	if strings.Contains(rawName, ", ") {
		split = strings.Split(rawName, ", ")
		firstName, lastName = split[1], split[0]
	} else if strings.Contains(rawName, ",") {
		split = strings.Split(rawName, ",")
		firstName, lastName = split[1], split[0]
	} else {
		split = strings.Split(rawName, " ")
		firstName, lastName = split[0], split[1]
	}
	if len(split) != 2 {
		fmt.Println("Unexpected name:")
		fmt.Println(rawName)
		fmt.Println(rawClubName)
		return "", "", ""
	}

	return firstName, lastName, rawClubName
}
