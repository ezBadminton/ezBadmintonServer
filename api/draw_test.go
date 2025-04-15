package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDraw(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	modeSettingsCName := CName[TournamentModeSettings]()
	competitionCName := CName[Competition]()
	tournamentPlanCName := "tournament_plans"

	app := newPersistentTestApp(t)
	defer app.Cleanup()

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	players := createTestPlayers(t, 4)
	competition := createTestCompetition(t, 1, Female)

	modeSettings, _ := NewProxy[TournamentModeSettings](app)
	modeSettings.SetType(SingleElimination)
	modeSettings.SetSeedingMode(TieredSeeds)
	modeSettings.SetWinningPoints(21)
	modeSettings.SetWinningSets(2)
	modeSettings.SetMaxPoints(30)
	modeSettings.SetTwoPointMargin(true)
	modeSettings.SetRaw("competitions", []string{competition.Id})
	modeSettings.WithCustomData(true)

	base_url := fmt.Sprintf("/api/ezbadminton/admin/draw/%v", competition.Id)

	secenarios := []*tests.ApiScenario{
		{
			Name:            "try to make draw without tournament mode settings",
			Method:          http.MethodPost,
			URL:             base_url + "/make",
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"no tournament mode settings"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:            "set tournament mode settings",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%v/records", modeSettingsCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, modeSettings),
			ExpectedStatus:  200,
			ExpectedContent: []string{modeSettingsCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "try to make draw with no registered teams",
			Method:          http.MethodPost,
			URL:             base_url + "/make",
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"no draw"},
			TestAppFactory:  newPersistentTestApp,
		},
	}
	for _, scenario := range secenarios {
		scenario.Test(t)
	}

	// Register players
	for _, player := range players {
		scenario := commonScenarios.registerTeam(headers, competition, []*Player{player})
		scenario.Test(t)
	}

	secenarios = []*tests.ApiScenario{
		{
			Name:            "try to make draw with no teams attending",
			Method:          http.MethodPost,
			URL:             base_url + "/make",
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"no draw"},
			TestAppFactory:  newPersistentTestApp,
		},
	}
	for _, scenario := range secenarios {
		scenario.Test(t)
	}

	for _, player := range players {
		scenario := tests.ApiScenario{
			Name:    "mark player as attending",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/playerstatus/%v", player.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"status": %v,
				"competitions": []
			}`, Attending)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		}
		scenario.Test(t)
	}

	var drawIds []string
	var drawRng int

	secenarios = []*tests.ApiScenario{
		{
			Name:            "fetch tournament plans before draw is made",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", tournamentPlanCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				plans := response["items"].([]any)
				if len(plans) != 0 {
					t.Fatal("a tournament plan exists despite no draw existing yet")
				}
			},
		},
		{
			Name:           "make draw",
			Method:         http.MethodPost,
			URL:            base_url + "/make",
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch competition with draw",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competition := unmarshalRecords[Competition](app, res)[0]
				drawRng = competition.RngSeed()
				drawIds = competition.Get("draw").([]string)
				if len(drawIds) != len(players) {
					t.Fatal("the draw does not contain the expected number of teams")
				}
			},
		},
		{
			Name:            "fetch tournament plans after draw was made",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", tournamentPlanCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				plans := response["items"].([]any)
				if len(plans) != 1 {
					t.Fatal("no tournament plan exists after a draw was made")
				}
				plan := plans[0].(map[string]any)
				eq0 := plan["started"].(bool) == false
				eq1 := plan["ended"].(bool) == false
				eq2 := plan["competition"].(string) == competition.Id
				if !eq0 || !eq1 || !eq2 {
					t.Fatal("the tournament plan is not in a valid initial state")
				}
				tournament := plan["tournament"].(map[string]any)
				entries := tournament["entries"].([]any)
				if len(entries) != len(players) {
					t.Fatal("the tournament plan does not have the expected number of entries")
				}
			},
		},
	}
	for _, scenario := range secenarios {
		scenario.Test(t)
	}

	secenarios = []*tests.ApiScenario{
		{
			Name:    "swap draw positions",
			Method:  http.MethodPost,
			URL:     base_url + "/swap",
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"swap": ["%v", "%v"]
			}`, drawIds[0], drawIds[1])),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch competition with swapped draw",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competition := unmarshalRecords[Competition](app, res)[0]
				swappedDrawIds := competition.Get("draw").([]string)
				eq0 := drawIds[0] == swappedDrawIds[1]
				eq1 := drawIds[1] == swappedDrawIds[0]
				eq2 := drawIds[2] == swappedDrawIds[2] && drawIds[3] == swappedDrawIds[3]
				if !eq0 || !eq1 || !eq2 {
					t.Fatal("the draw swap was unsucessful")
				}
			},
		},
		{
			Name:           "redraw",
			Method:         http.MethodPost,
			URL:            base_url + "/redraw",
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch competition after redraw",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competition := unmarshalRecords[Competition](app, res)[0]
				if competition.RngSeed() == drawRng {
					t.Fatal("the draw RNG did not change for the redraw")
				}
			},
		},
		{
			Name:    "try to set unknown team as seed",
			Method:  http.MethodPost,
			URL:     base_url + "/seeds",
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"seeds": ["%v", "%v"]
			}`, drawIds[2], "unknownteam")),
			ExpectedStatus:  400,
			ExpectedContent: []string{"does not exist"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "set seeds",
			Method:  http.MethodPost,
			URL:     base_url + "/seeds",
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"seeds": ["%v", "%v"]
			}`, drawIds[2], drawIds[3])),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "redraw with seeds",
			Method:         http.MethodPost,
			URL:            base_url + "/redraw",
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch competition after redraw with seeds",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competition := unmarshalRecords[Competition](app, res)[0]
				seededDrawIds := competition.Get("draw").([]string)
				eq0 := seededDrawIds[0] == drawIds[2]
				eq1 := seededDrawIds[1] == drawIds[3]
				if !eq0 || !eq1 {
					t.Fatal("the seeded teams did not end up at the top of the draw")
				}
			},
		},
		{
			Name:           "delete draw",
			Method:         http.MethodDelete,
			URL:            base_url,
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch competition after draw delete",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competition := unmarshalRecords[Competition](app, res)[0]
				drawRng = competition.RngSeed()
				drawIds = competition.Get("draw").([]string)
				if len(drawIds) != 0 {
					t.Fatal("the draw is not deleted")
				}
			},
		},
		{
			Name:            "fetch tournament plans after draw was deleted",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%v/records", tournamentPlanCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				plans := response["items"].([]any)
				if len(plans) != 0 {
					t.Fatal("the tournament plan is not deleted after the draw was deleted")
				}
			},
		},
	}
	for _, scenario := range secenarios {
		scenario.Test(t)
	}
}
