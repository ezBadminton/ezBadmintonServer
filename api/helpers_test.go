package api_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/hooks"
	"github.com/pocketbase/pocketbase/tests"
)

const testDataPath = "../api_test/test_data"

var persistedDataPath = ""

func newTestApp(t testing.TB) *tests.TestApp {
	app, err := tests.NewTestApp(testDataPath)
	if err != nil {
		t.Fatal(err)
	}
	hooks.InitHooksAndApi(app)
	return app
}

func newPersistentTestApp(t testing.TB) *tests.TestApp {
	dataPath := persistedDataPath
	if dataPath == "" {
		dataPath = testDataPath
	}
	app, err := tests.NewTestApp(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	hooks.InitHooksAndApi(app)
	return app
}

func persistTestData(t testing.TB, app *tests.TestApp, res *http.Response) {
	currentDataDir := app.DataDir()
	tempPath, err := tests.TempDirClone(currentDataDir)
	if err != nil {
		t.Fatal("Failed to persist test data")
	}
	cleanUpPersistedTestData(t, app, res)
	persistedDataPath = tempPath
}

func cleanUpPersistedTestData(t testing.TB, app *tests.TestApp, res *http.Response) {
	if persistedDataPath != "" {
		if err := os.RemoveAll(persistedDataPath); err != nil {
			t.Fatal(err)
		}
	}
	persistedDataPath = ""
}

var commonScenarios commonTestScenarios

type commonTestScenarios struct {
	registerOrganizer tests.ApiScenario
}

func init() {
	organizerCName := CName[TournamentOrganizer]()

	commonScenarios = commonTestScenarios{
		registerOrganizer: tests.ApiScenario{
			Name:   "register tournament organizer",
			Method: http.MethodPost,
			URL:    fmt.Sprintf("/api/collections/%s/records", organizerCName),
			Body: strings.NewReader(`{
				"username": "testuser",
				"password": "12345",
				"passwordConfirm": "12345"
			}`),
			ExpectedStatus:  200,
			ExpectedContent: []string{organizerCName, "testuser"},
			TestAppFactory:  newPersistentTestApp,
			AfterTestFunc:   persistTestData,
		},
	}
}
