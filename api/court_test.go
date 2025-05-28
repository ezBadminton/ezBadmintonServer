package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestConventionalCourtAssignment(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	settings := testCompetitionSettings{
		tType:    SingleElimination,
		numTeams: 4,
		teamSize: 1,
	}

	competition, teams, players, tPlan := commonScenarios.setUpTestTournament(settings, t, headers)
	_, _, _, _ = competition, teams, players, tPlan

	rounds := fetchSingleEliminationRounds(t, tPlan)
	semi1 := rounds[0][0]
	semi2 := rounds[0][1]
	final := rounds[1][0]
	_, _, _ = semi1, semi2, final

	courts := setUpCourts(t, 2)

	scenarios := []tests.ApiScenario{
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
			Name:    "try to assign a second match to the same court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi2.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"occupied"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "try to assign the match to a second court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[1].Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"not in the CourtWait status"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:           "assign match to court automatically",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi2.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				persistTestData(t, app, nil)
				rounds := fetchSingleEliminationRounds(t, tPlan)
				semi2 := rounds[0][1]
				if semi2.Court().Id != courts[1].Id {
					t.Fatal("the automatically assigned court is not the expected one")
				}
			},
		},
		{
			Name:           "start match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/start", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "start match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/start", semi2.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "set match score",
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
			Name:            "try to assign court to match before qualifications are complete",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", final.Id),
			Headers:         headers,
			ExpectedStatus:  400,
			ExpectedContent: []string{"not in the CourtWait status"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "set match score",
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
			Name:    "assign match to court after qualifications are complete",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", final.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "unassign match from court",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/courts/%v/unassign", final.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "re-assign match to court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", final.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

// An implicit court assignment happens when a match is reset and
// therefore put back onto the court that it was assigned to
func TestImplicitCourtAssignment(t *testing.T) {
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

	scenarios := []tests.ApiScenario{
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
			Name:           "start match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/start", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "set match score",
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
			Name:           "reset match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/reset", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "try to assign match after reset assignment",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi2.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"occupied"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:           "start match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/start", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "set match score",
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
			Name:    "assign match to court that just had anotherm match finished",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi2.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[0].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "reset match after the court has another assigned match",
			Method:         http.MethodPost,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/matches/%v/reset", semi1.Id),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "assign the reset match to a different court",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/courts/%v/assign", semi1.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"court": "%v"
			}`, courts[1].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
