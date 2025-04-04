package api_test

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/tests"
)

func TestCompetitionCreation(t *testing.T) {
	defer cleanUpPersistedTestData(t)

	competitionCName := CName[Competition]()

	app := newPersistentTestApp(t)
	defer app.Cleanup()

	mensSingles, _ := NewProxy[Competition](app)
	mensSingles.SetTeamSize(1)
	mensSingles.SetGenderCategory(Male)

	womensDoubles, _ := NewProxy[Competition](app)
	womensDoubles.SetTeamSize(2)
	womensDoubles.SetGenderCategory(Female)

	commonScenarios.registerOrganizer.Test(t)

	headers := authHeader(generateAuthorization(t))

	scenarios := []tests.ApiScenario{
		{
			Name:            "try to create competition without authorization",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", competitionCName),
			Body:            recordToRequestBody(t, mensSingles),
			ExpectedStatus:  400,
			ExpectedContent: []string{"data"},
			TestAppFactory:  newPersistentTestApp,
		},
		{
			Name:            "create competition",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", competitionCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, mensSingles),
			ExpectedStatus:  200,
			ExpectedContent: []string{competitionCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "create a second competition",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", competitionCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, womensDoubles),
			ExpectedStatus:  200,
			ExpectedContent: []string{competitionCName},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
		{
			Name:            "try to re-create an existing competition",
			Method:          http.MethodPost,
			URL:             fmt.Sprintf("/api/collections/%s/records", competitionCName),
			Headers:         headers,
			Body:            recordToRequestBody(t, mensSingles),
			ExpectedStatus:  400,
			ExpectedContent: []string{"data"},
			TestAppFactory:  newPersistentTestApp,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
