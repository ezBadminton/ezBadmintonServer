package api_test

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"

	"github.com/pocketbase/pocketbase/tests"
)

func TestOrganizerRegistration(t *testing.T) {
	organizerCName := CName[TournamentOrganizer]()

	scenarios := []tests.ApiScenario{
		{
			Name:            "does tournament organizer account exist in new database",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/ezbadminton/%s/exists", organizerCName),
			ExpectedStatus:  200,
			ExpectedContent: []string{"OrganizerUserExists", "false"},
			TestAppFactory:  newTestApp,
		},
		commonScenarios.registerOrganizer,
		{
			Name:            "does tournament organizer account exist after registration",
			Method:          http.MethodGet,
			URL:             fmt.Sprintf("/api/ezbadminton/%s/exists", organizerCName),
			ExpectedStatus:  200,
			ExpectedContent: []string{"OrganizerUserExists", "true"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   cleanUpPersistedTestData,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
