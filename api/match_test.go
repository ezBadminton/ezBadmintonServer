package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMatchControls(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	settings := testCompetitionSettings{
		tType:    SingleElimination,
		numTeams: 4,
		teamSize: 1,
	}

	_, _, _, tPlan := commonScenarios.setUpTestTournament(settings, t, headers)

	rounds := fetchSingleEliminationRounds(t, tPlan)
	semi1 := rounds[0][0]
	semi2 := rounds[0][1]

	courts := setUpCourts(t, 2)
	baseUrl := "/api/ezbadminton/admin/matches"

	scenarios := []tests.ApiScenario{
		{
			Name:            "try to start match before court assignment",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("%v/%v/start", baseUrl, semi1.Id),
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"can not be started"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "assign match to court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "start match after court assignment",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("%v/%v/start", baseUrl, semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "try to cancel match that is not running",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("%v/%v/cancel", baseUrl, semi2.Id),
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"can not be canceled"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:           "cancel match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("%v/%v/cancel", baseUrl, semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "try to end match that is not running",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("%v/%v/end", baseUrl, semi1.Id),
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"can not be ended"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:           "start match again after cancel",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("%v/%v/start", baseUrl, semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "assign match to court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi2.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[1].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "start match after court assignment",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("%v/%v/start", baseUrl, semi2.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "end match without score",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("%v/%v/end", baseUrl, semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "end match with score",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/matches/%v/score", semi2.Id),
			Headers: headers,
			Body: strings.NewReader(`{
				"team1points": [21, 21],
				"team2points": [0, 0]
			}`),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "set score after match was ended",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/matches/%v/score", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(`{
				"team1points": [21, 21],
				"team2points": [0, 0]
			}`),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "edit score",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/matches/%v/score", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(`{
				"team1points": [21, 21],
				"team2points": [1, 0]
			}`),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "try to submit a malformed score",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/matches/%v/score", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(`{
				"team1points": [22, 21],
				"team2points": [1, 0]
			}`),
			ExpectedStatus:  400,
			ExpectedContent: []string{"margin is invalid"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:           "reset match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/reset", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "try to reset again",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/ezbadminton/admin/matches/%v/reset", semi1.Id),
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"can not have its score reset"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "mark match as printed",
			Method:  http.MethodPost,
			URL:     "/api/ezbadminton/admin/matches/markprint",
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"matches": ["%v"]
			}`, semi1.Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
