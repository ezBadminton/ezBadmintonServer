package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestPlayerAdd(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	playerCName := CName[Player]()
	clubCName := CName[Club]()

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	app := newPersistentTestApp(t)
	defer app.Cleanup()

	player, _ := NewProxy[Player](app)
	player.SetFirstName("Bob")
	player.SetLastName("Alice")
	player.SetStatus(NotAttending)

	playerWithClub, _ := NewProxy[Player](app)
	playerWithClub.SetFirstName("Mary")
	playerWithClub.SetLastName("Jane")
	playerWithClub.SetStatus(NotAttending)
	playerWithClub.SetRaw("club", "newclub:testclub")

	var club *Club

	scenarios := []*tests.ApiScenario{
		{
			Name:            "try to add player without authorization",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Body:            recordToRequestBody(t, player),
			ExpectedStatus:  400,
			ExpectedContent: []string{"data"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:            "add new player",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, player),
			ExpectedStatus:  200,
			ExpectedContent: []string{playerCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "add new player with new club",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, playerWithClub),
			ExpectedStatus:  200,
			ExpectedContent: []string{playerCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "fetch players",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				players := unmarshalRecords[Player](app, res)
				if len(players) != 2 {
					t.Fatal("the new player is not in the fetch result")
				}
				if players[0].FirstName() != player.FirstName() || players[0].LastName() != player.LastName() {
					t.Fatal("the new player does not have the name that was set")
				}
			},
		},
		{
			Name:            "fetch clubs",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", clubCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				clubs := unmarshalRecords[Club](app, res)
				if len(clubs) != 1 {
					t.Fatal("the new club is not in the fetch result")
				}
				if clubs[0].Name() != "testclub" {
					t.Fatal("the new club does not have the name that was set")
				}
				club = clubs[0]
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}

	playerWithExistingClub, _ := NewProxy[Player](app)
	playerWithExistingClub.SetFirstName("Andy")
	playerWithExistingClub.SetLastName("Tecis")
	playerWithExistingClub.SetStatus(NotAttending)
	playerWithExistingClub.SetClub(club)

	scenarios = []*tests.ApiScenario{
		{
			Name:            "add new player with existing club",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, playerWithExistingClub),
			ExpectedStatus:  200,
			ExpectedContent: []string{playerCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "fetch existing club player",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				players := unmarshalRecords[Player](app, res)
				if len(players) != 3 {
					t.Fatal("the new players are not in the fetch result")
				}
				if players[2].Get("club") != club.Id {
					t.Fatal("the new player does not have the club that was set")
				}
			},
		},
		{
			Name:            "fetch clubs after player was added to existing club",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", clubCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				clubs := unmarshalRecords[Club](app, res)
				if len(clubs) != 1 {
					t.Fatal("the new club is not in the fetch result")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestTeamRegistration(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	players := createTestPlayers(t, 6, NotAttending)
	singles := createTestCompetition(t, 1, Female)
	doubles := createTestCompetition(t, 2, Male)

	var doublesTeamId string

	playerCName := CName[Player]()
	competitionCName := CName[Competition]()

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	scenarios := []tests.ApiScenario{
		{
			Name:            "fetch players",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", playerCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				players := unmarshalRecords[Player](app, res)
				if len(players) != 6 {
					t.Fatal("the test players were not created")
				}
			},
		},
		{
			Name:            "fetch competition",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", competitionCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				competitions := unmarshalRecords[Competition](app, res)
				if len(competitions) != 2 {
					t.Fatal("the test competitions were not created")
				}
			},
		},
		{
			Name:            "fetch registrations of new competition",
			Method:          http.MethodGet,
			URL:             "/api/collections/registrations/records",
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				registrations := response["items"].([]any)
				if len(registrations) != 0 {
					t.Fatal("the initial registrations are not empty")
				}
			},
		},
		{
			Name:    "register a singles team",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", singles.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"players": ["%v"]
			}`, players[0].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "try to register a singles player twice",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", singles.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"players": ["%v"]
			}`, players[0].Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"already registered"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:    "register a doubles team",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", doubles.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"players": ["%v", "%v"]
			}`, players[1].Id, players[2].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:    "try to register a doubles player twice",
			Method:  http.MethodPost,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", doubles.Id),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"players": ["%v", "%v"]
			}`, players[2].Id, players[3].Id)),
			ExpectedStatus:  400,
			ExpectedContent: []string{"already registered"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:            "fetch registrations",
			Method:          http.MethodGet,
			URL:             "/api/collections/registrations/records",
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				registrations := response["items"].([]any)
				if len(registrations) != 2 {
					t.Fatal("the registrations are not present")
				}
				doublesTeamId = registrations[1].(map[string]any)["team"].(string)
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}

	scenarios = []tests.ApiScenario{
		{
			Name:    "edit a doubles team",
			Method:  http.MethodPatch,
			URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", doublesTeamId),
			Headers: headers,
			Body: strings.NewReader(fmt.Sprintf(`{
				"players": ["%v", "%v"]
			}`, players[1].Id, players[2].Id)),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:           "delete a team",
			Method:         http.MethodDelete,
			URL:            fmt.Sprintf("/api/ezbadminton/admin/registration/%v", doublesTeamId),
			Headers:        headers,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch registrations after delete",
			Method:          http.MethodGet,
			URL:             "/api/collections/registrations/records",
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				response := unmarshalListRespose(res)
				registrations := response["items"].([]any)
				if len(registrations) != 1 {
					t.Fatal("the registration was not deleted")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func createTestPlayers(t testing.TB, amount int, status PlayerStatus) []*Player {
	app := newPersistentTestApp(t)
	defer app.Cleanup()

	players := make([]*Player, amount)

	for i := range amount {
		player, _ := NewProxy[Player](app)
		player.SetFirstName(fmt.Sprintf("%v", i))
		player.SetLastName(fmt.Sprintf("-%v-", i))
		player.SetStatus(status)
		if err := app.Save(player); err != nil {
			t.Fatal(err)
		}
		players[i] = player
	}

	persistTestData(t, app, nil)

	return players
}
