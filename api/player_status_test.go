package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

// Make a test tournament (2 semi finals, 1 final) and change a player's status
// to be withdrawn. Check if the tournament reacts correctly by giving an automatic win
// to the opponent.
func TestPlayerStatusChanges(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	settings := testCompetitionSettings{
		tType:    SingleElimination,
		numTeams: 4,
		teamSize: 1,
	}

	competition, teams, players, tPlan := commonScenarios.setUpTestTournament(settings, t, headers)

	rounds := fetchSingleEliminationRounds(t, tPlan)
	finals := rounds[1][0]

	var withdrawnMatchId string
	previewScenario := tests.ApiScenario{
		Name:            "get status change preview",
		Method:          http.MethodGet,
		URL:             fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v/%v/preview", players[0].Id, NotAttending),
		Headers:         headers,
		ExpectedStatus:  200,
		ExpectedContent: []string{"changes"},
		TestAppFactory:  newPersistentTestApp,
		AfterTestFunc: func(tb testing.TB, app *tests.TestApp, res *http.Response) {
			response := unmarshalJsonResponse(res)
			changes := response["changes"].(map[string]any)
			keys := make([]string, 0, len(changes))
			for k := range changes {
				keys = append(keys, k)
			}
			if len(keys) != 1 {
				t.Fatal("the preview contains more than one competition despite the player only being registered to one")
			}
			previewCompetitionId := keys[0]
			if previewCompetitionId != competition.Id {
				t.Fatal("the ID of the competition in the preview is wrong")
			}
			withdrawnMatchId = changes[previewCompetitionId].([]any)[0].(string)
		},
	}
	previewScenario.Test(t)

	var opponentOfWithdrawn string
	slot1, slot2 := matchupFromMatchData(t, headers, withdrawnMatchId)
	if slot1 == teams[0].Id {
		opponentOfWithdrawn = slot2
	} else {
		opponentOfWithdrawn = slot1
	}

	withdrawScenario := tests.ApiScenario{
		Name:    "withdraw player by setting their status to not attending",
		Method:  http.MethodPost,
		URL:     fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v", players[0].Id),
		Headers: headers,
		Body: strings.NewReader(fmt.Sprintf(`{
				"competitions": ["%v"],
				"status": %v
			}`, competition.Id, NotAttending)),
		ExpectedStatus: 200,
		TestAppFactory: newPersistentTestApp,
		AfterTestFunc:  persistTestData,
	}
	withdrawScenario.Test(t)

	slot1, slot2 = matchupFromMatchData(t, headers, finals.Id)
	if opponentOfWithdrawn != slot1 && opponentOfWithdrawn != slot2 {
		t.Fatal("the opponent of the withdrawn player did not advance to the final")
	}

	reenterScenario := tests.ApiScenario{
		Name:    "reenter the player who previously withdrew",
		Method:  http.MethodPost,
		URL:     fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v", players[0].Id),
		Headers: headers,
		Body: strings.NewReader(fmt.Sprintf(`{
				"competitions": ["%v"],
				"status": %v
			}`, competition.Id, Attending)),
		ExpectedStatus: 200,
		TestAppFactory: newPersistentTestApp,
		AfterTestFunc:  persistTestData,
	}
	reenterScenario.Test(t)

	slot1, slot2 = matchupFromMatchData(t, headers, finals.Id)
	if "" != slot1 || "" != slot2 {
		t.Fatal("the final slots did not revert back to empty after the withdrawal was undone")
	}

	scenarios := []tests.ApiScenario{
		{
			Name:    "try to submit an invalid status",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v", players[0].Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"competitions": ["%v"],
				"status": 99
			}`, competition.Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"Invalid player status"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "try to submit an invalid status",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v", players[0].Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"competitions": ["%v"],
				"status": -1
			}`, competition.Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"Invalid player status"},
			TestAppFactory:  newPersistentTestApp,
		},
	}
	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
