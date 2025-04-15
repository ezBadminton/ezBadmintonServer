package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestPlayingLevelOrder(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	playingLevelCName := CName[PlayingLevel]()

	app := newPersistentTestApp(t)
	defer app.Cleanup()

	commonScenarios.registerOrganizer().Test(t)
	headers := authHeader(generateAuthorization(t))

	playingLevel0, _ := NewProxy[PlayingLevel](app)
	playingLevel0.SetName("pl0")
	playingLevel1, _ := NewProxy[PlayingLevel](app)
	playingLevel1.SetName("pl1")
	playingLevel2, _ := NewProxy[PlayingLevel](app)
	playingLevel2.SetName("pl2")

	scenarios := []*tests.ApiScenario{
		{
			Name:            "add playing level 0",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playingLevelCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, playingLevel0),
			ExpectedStatus:  200,
			ExpectedContent: []string{playingLevelCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "add playing level 1",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playingLevelCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, playingLevel1),
			ExpectedStatus:  200,
			ExpectedContent: []string{playingLevelCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "add playing level 2",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", playingLevelCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, playingLevel2),
			ExpectedStatus:  200,
			ExpectedContent: []string{playingLevelCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "fetch playing levels",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", playingLevelCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				playingLevels := unmarshalRecords[PlayingLevel](app, res)
				if len(playingLevels) != 3 {
					t.Fatal("the new playing levels are not in the fetch result")
				}
				eq0 := playingLevels[0].Name() == playingLevel0.Name()
				eq1 := playingLevels[1].Name() == playingLevel1.Name()
				eq2 := playingLevels[2].Name() == playingLevel2.Name()
				if !eq0 || !eq1 || !eq2 {
					t.Fatal("the new playing levels do not have the names that were set")
				}

				eq0 = playingLevels[0].Index() == 0
				eq1 = playingLevels[1].Index() == 1
				eq2 = playingLevels[2].Index() == 2
				if !eq0 || !eq1 || !eq2 {
					t.Fatal("the new playing levels do not have the expected indices")
				}
			},
		},
		{
			Name:    "reorder playing level indices",
			Method:  http.MethodPost,
			URL:     "/api/ezbadminton/admin/playinglevels/reorder",
			Headers: headers,
			Body: strings.NewReader(`{
				"from": 0,
				"to": 2
			}`),
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		},
		{
			Name:            "fetch reordered playing levels",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/collections/%s/records", playingLevelCName),
			Headers:         headers,
			ExpectedStatus:  200,
			ExpectedContent: []string{"items"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				playingLevels := unmarshalRecords[PlayingLevel](app, res)
				if len(playingLevels) != 3 {
					t.Fatal("the new playing levels are not in the fetch result")
				}
				eq0 := playingLevels[0].Index() == 2
				eq1 := playingLevels[1].Index() == 0
				eq2 := playingLevels[2].Index() == 1
				if !eq0 || !eq1 || !eq2 {
					t.Fatal("the new playing levels do not have the expected reordered indices")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
