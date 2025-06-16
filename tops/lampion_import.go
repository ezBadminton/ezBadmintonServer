package tops

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

type LampionImporter struct {
	App core.App
	Url string
}

type parsedPlayer struct {
	FirstName, LastName, ClubName string
	player                        *Player
}

type parsedRegistration struct {
	Discipline   string
	PlayingLevel string
	Players      []*parsedPlayer
	Seed         int
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

	playingLevelStore, _ := store.FindRecordStore[PlayingLevel]()
	levels := playingLevelStore.ListRecords()

	for i, name := range levelNames {
		level, _ := NewProxy[PlayingLevel](l.App)
		level.SetIndex(i)
		level.SetName(name)
		if err := l.App.Save(level); err == nil {
			levels = append(levels, level)
		}
	}

	for _, level := range levels {
		for _, competition := range competitions {
			competition := Clone(competition)
			competition.SetPlayingLevel(level)
			l.App.Save(competition)
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

	parsedPlayers, parsedRegistrations := l.parsePlayers(rawEntries)
	playersToCreate, playersToDelete := l.filterPlayers(parsedPlayers)
	clubs, err := l.createClubs(playersToCreate)
	if err != nil {
		return err
	}

	err = l.createPlayers(playersToCreate, clubs)
	if err != nil {
		return err
	}

	err = l.deletePlayers(playersToDelete)
	if err != nil {
		return err
	}

	err = l.createAndRegisterTeams(parsedRegistrations)
	return err
}

func (l *LampionImporter) parsePlayers(rawEntries []any) (map[string]*parsedPlayer, []*parsedRegistration) {
	playerSet := make(map[string]*parsedPlayer)
	registrations := make([]*parsedRegistration, 0)

	for _, entry := range rawEntries {
		entry := entry.(map[string]any)
		parsedEntries := make([]*parsedPlayer, 0)
		for _, playerIndex := range []string{"1", "2"} {
			rawName := entry[fmt.Sprintf("Spieler%v", playerIndex)].(string)
			rawClubName := entry[fmt.Sprintf("Verein%v", playerIndex)].(string)
			firstName, lastName, clubName := l.parsePlayer(rawName, rawClubName)
			if firstName == "" {
				continue
			}
			player, ok := playerSet[firstName+lastName+clubName]
			if ok {
				parsedEntries = append(parsedEntries, player)
				continue
			}
			player = &parsedPlayer{
				FirstName: firstName,
				LastName:  lastName,
				ClubName:  clubName,
			}
			parsedEntries = append(parsedEntries, player)
		}

		if len(parsedEntries) == 0 {
			continue
		}

		discipline := entry["Disziplin"].(string)[:2]
		playingLevel := entry["Spielklasse"].(string)

		seed := 0
		seedString, ok := entry["Setzplatz"].(string)
		if ok {
			seed, _ = strconv.Atoi(seedString)
		}

		registration := &parsedRegistration{
			Discipline:   discipline,
			PlayingLevel: playingLevel,
			Players:      parsedEntries,
			Seed:         seed,
		}

		registrations = append(registrations, registration)
		for _, player := range parsedEntries {
			playerSet[player.FirstName+player.LastName+player.ClubName] = player
		}
	}

	return playerSet, registrations
}

// Separates the players into players that need to be created and players
// that need to be deleted
func (l *LampionImporter) filterPlayers(importedPlayers map[string]*parsedPlayer) ([]*parsedPlayer, []*Player) {
	competitionStore, _ := store.FindRecordStore[Competition]()
	playerStore, _ := store.FindRecordStore[Player]()

	existingPlayers := make(map[string]*Player)
	playersInCompetitions := make(map[string]*Player)

	for _, player := range playerStore.RecordList {
		firstName := player.FirstName()
		lastName := player.LastName()
		clubName := ""
		if player.Club() != nil {
			clubName = player.Club().Name()
		}

		existingPlayers[firstName+lastName+clubName] = player
	}

	for _, competition := range competitionStore.RecordList {
		if len(competition.Matches()) == 0 {
			continue
		}

		for _, team := range competition.Draw() {
			for _, player := range team.Players() {
				firstName := player.FirstName()
				lastName := player.LastName()
				clubName := ""
				if player.Club() != nil {
					clubName = player.Club().Name()
				}

				playersInCompetitions[firstName+lastName+clubName] = player
			}
		}
	}

	for key, importedPlayer := range importedPlayers {
		player, exists := existingPlayers[key]
		if exists {
			importedPlayer.player = player
			delete(importedPlayers, key)
			delete(existingPlayers, key)
		}
	}

	for key := range playersInCompetitions {
		delete(existingPlayers, key)
	}

	playersToCreate := make([]*parsedPlayer, 0)
	playersToDelete := make([]*Player, 0)

	for _, player := range importedPlayers {
		playersToCreate = append(playersToCreate, player)
	}
	for _, player := range existingPlayers {
		playersToDelete = append(playersToDelete, player)
	}

	return playersToCreate, playersToDelete
}

func (l *LampionImporter) createClubs(players []*parsedPlayer) (map[string]*Club, error) {
	nameSet := map[string]any{}

	for _, parsedPlayer := range players {
		clubName := parsedPlayer.ClubName
		if clubName != "" {
			nameSet[clubName] = struct{}{}
		}
	}

	clubs := make(map[string]*Club)

	clubStore, _ := store.FindRecordStore[Club]()
	for _, club := range clubStore.RecordList {
		clubs[club.Name()] = club
	}

	for clubName := range nameSet {
		_, exists := clubs[clubName]
		if exists {
			continue
		}
		club, _ := NewProxy[Club](l.App)
		club.SetName(clubName)
		if err := l.App.Save(club); err != nil {
			return nil, err
		}
		clubs[clubName] = club
	}

	return clubs, nil
}

func (l *LampionImporter) createPlayers(importedPlayers []*parsedPlayer, clubs map[string]*Club) error {
	for _, importedPlayer := range importedPlayers {
		club, _ := clubs[importedPlayer.ClubName]
		player, _ := NewProxy[Player](l.App)
		player.SetStatus(NotAttending)
		player.SetFirstName(importedPlayer.FirstName)
		player.SetLastName(importedPlayer.LastName)
		player.SetClub(club)

		if err := l.App.Save(player); err != nil {
			return err
		}

		importedPlayer.player = player
	}
	return nil
}

func (l *LampionImporter) deletePlayers(players []*Player) error {
	for _, player := range players {
		if err := l.App.Delete(player); err != nil {
			return err
		}
	}
	return nil
}

func (l *LampionImporter) createAndRegisterTeams(importedRegistrations []*parsedRegistration) error {
	registrations := make(map[string][]*parsedRegistration)

	for _, registration := range importedRegistrations {
		key := registration.Discipline + registration.PlayingLevel
		_, ok := registrations[key]
		if !ok {
			registrations[key] = make([]*parsedRegistration, 0)
		}
		registrations[key] = append(registrations[key], registration)
	}

	competitionStore, _ := store.FindRecordStore[Competition]()
	for _, competition := range competitionStore.RecordList {
		if len(competition.Matches()) != 0 {
			continue
		}

		competition = Clone(competition)

		for _, team := range competition.Registrations() {
			if err := tops.registrationStore.deleteTeam(l.App, team); err != nil {
				return err
			}
		}
		competition.SetRegistrations([]*Team{})
		competition.SetDraw([]*Team{})
		competition.SetSeeds([]*Team{})

		key := ""
		switch {
		case competition.TeamSize() == 1 && competition.GenderCategory() == Female:
			key = "DE"
		case competition.TeamSize() == 1 && competition.GenderCategory() == Male:
			key = "HE"
		case competition.TeamSize() == 2 && competition.GenderCategory() == Female:
			key = "DD"
		case competition.TeamSize() == 2 && competition.GenderCategory() == Male:
			key = "HD"
		case competition.TeamSize() == 2 && competition.GenderCategory() == Mixed:
			key = "MD"
		}
		key = key + competition.PlayingLevel().Name()

		regs := registrations[key]
		seeds := make(map[*Team]int, 0)

		for _, registration := range regs {
			team, _ := NewProxy[Team](l.App)
			teamMembers := make([]*Player, 0, len(registration.Players))
			for _, importedPlayer := range registration.Players {
				teamMembers = append(teamMembers, importedPlayer.player)
			}
			team.SetPlayers(teamMembers)

			if err := tops.registrationStore.registerTeam(l.App, team, competition); err != nil {
				if strings.Contains(err.Error(), "already registered") {
					warnMsg := fmt.Sprintf("The player %v %v is registered multiple times.\n", teamMembers[0].FirstName(), teamMembers[0].LastName())
					l.App.Logger().Warn(warnMsg)
				} else {
					return err
				}
			}
			if registration.Seed > 0 {
				seeds[team] = registration.Seed
			}
		}

		if len(seeds) == 0 {
			continue
		}

		seededTeams := make([]*Team, 0, len(seeds))
		for team := range seeds {
			seededTeams = append(seededTeams, team)
		}

		slices.SortFunc(seededTeams, func(a, b *Team) int { return cmp.Compare(seeds[a], seeds[b]) })

		competition.SetSeeds(seededTeams)
		if err := l.App.Save(competition); err != nil {
			return err
		}
	}

	return nil
}

// Returns first name, last name, club name
func (l *LampionImporter) parsePlayer(rawName, rawClubName string) (string, string, string) {
	if rawName == "" || rawName == "frei" {
		return "", "", ""
	}
	var firstName, lastName string
	var split []string
	if strings.Contains(rawName, ", ") {
		split = strings.Split(rawName, ", ")
	} else if strings.Contains(rawName, ",") {
		split = strings.Split(rawName, ",")
	} else {
		split = strings.Split(rawName, " ")
		slices.Reverse(split)
	}
	if len(split) != 2 {
		warnMsg := fmt.Sprintf("Unexpected name:\n%v\n%v\n", rawName, rawClubName)
		l.App.Logger().Warn(warnMsg)
		return "", "", ""
	}

	firstName, lastName = split[1], split[0]

	return firstName, lastName, rawClubName
}
