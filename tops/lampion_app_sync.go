package tops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
)

type LampionSync struct {
	Url    string
	ApiKey string
}

func newLampionSync(matchManager *MatchManager) *LampionSync {
	apiKey := os.Getenv("LAMPION_API_KEY")
	if apiKey == "" {
		return nil
	}
	sync := &LampionSync{
		Url:    "https://lampionturnier-app.tgcamberg1848.de/api",
		ApiKey: apiKey,
	}
	matchManager.onStart.Bind(priorityHandler(sync.handleMatchStart, -5))
	matchManager.onScoreSet.Bind(priorityHandler(sync.handleMatchScore, -5))

	return sync
}

func (l *LampionSync) handleMatchStart(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	go l.fireAndForgetMatchMessage(e)

	return nil
}

func (l *LampionSync) handleMatchScore(e *ScoreEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	go l.fireAndForgetScoreMessage(e)

	return nil
}

func (l *LampionSync) fireAndForgetMatchMessage(e *MatchEvent) {
	tournament := tops.tournamentStore.byMatch[e.MatchData.Id]
	competitionAbbreviation := competitionToAbbreviation(tournament.Competition)
	courtName := e.MatchData.Court().Name()
	slot1Name := slotToName(e.Match.match.Slot1)
	slot2Name := slotToName(e.Match.match.Slot2)

	message := fmt.Sprintf("%v - %v\n%v : %v", competitionAbbreviation, courtName, slot1Name, slot2Name)

	body := map[string]any{
		"from":        "Turnierleitung",
		"message":     message,
		"messageType": "MATCH",
	}

	l.sendMessage(body)
}

func (l *LampionSync) fireAndForgetScoreMessage(e *ScoreEvent) {
	tournament := tops.tournamentStore.byMatch[e.MatchData.Id]
	competitionAbbreviation := competitionToAbbreviation(tournament.Competition)
	slot1Name := slotToName(e.Match.match.Slot1)
	slot2Name := slotToName(e.Match.match.Slot2)
	scoreString := scoreToString(e.ScoreData)

	message := fmt.Sprintf("%v\n%v : %v (%v)", competitionAbbreviation, slot1Name, slot2Name, scoreString)

	body := map[string]any{
		"from":        "Turnierleitung",
		"message":     message,
		"messageType": "MATCHRESULT",
	}

	l.sendMessage(body)
}

func (l *LampionSync) sendMessage(body map[string]any) {
	buf := &bytes.Buffer{}
	json.NewEncoder(buf).Encode(body)

	req, err := http.NewRequest("POST", l.Url+"/news/send", buf)
	if err != nil {
		return
	}
	req.Header.Add("Authorization", l.ApiKey)
	req.Header.Add("Content-Type", "application/json")

	http.DefaultClient.Do(req)
}

func slotToName(slot *got.Slot) string {
	team := slot.Player.(TournamentPlayer).Team
	players := team.Players()
	playerNames := make([]string, 0, len(players))
	for _, player := range players {
		playerName := fmt.Sprintf("%v, %v.", player.LastName(), player.FirstName()[:1])
		playerNames = append(playerNames, playerName)
	}
	name := strings.Join(playerNames, " / ")
	return name
}

func competitionToAbbreviation(competition *Competition) string {
	var discipline string
	switch {
	case competition.TeamSize() == 1 && competition.GenderCategory() == Female:
		discipline = "DE"
	case competition.TeamSize() == 1 && competition.GenderCategory() == Male:
		discipline = "HE"
	case competition.TeamSize() == 2 && competition.GenderCategory() == Female:
		discipline = "DD"
	case competition.TeamSize() == 2 && competition.GenderCategory() == Male:
		discipline = "HD"
	case competition.TeamSize() == 2 && competition.GenderCategory() == Mixed:
		discipline = "MD"
	}
	level := competition.PlayingLevel().Name()
	level = level[len(level)-1 : len(level)]

	return fmt.Sprintf("%v-%v", discipline, level)
}

func scoreToString(scoreData []*MatchSet) string {
	setStrings := make([]string, 0, len(scoreData))
	for _, set := range scoreData {
		p1, p2 := set.Team1Points(), set.Team2Points()
		setString := fmt.Sprintf("%v:%v", p1, p2)
		setStrings = append(setStrings, setString)
	}
	scoreString := strings.Join(setStrings, " | ")

	return scoreString
}
