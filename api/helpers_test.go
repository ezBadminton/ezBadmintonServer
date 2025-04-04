package api_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/hooks"
	"github.com/pocketbase/pocketbase/core"
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

// A persistent test app keeps its data across test scenarios
// as long as the previous scenarios call persistTestData
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

// Persist the data for the next persistent test app before the scenario is being cleaned up
func persistTestData(t testing.TB, app *tests.TestApp, res *http.Response) {
	currentDataDir := app.DataDir()
	tempPath, err := tests.TempDirClone(currentDataDir)
	if err != nil {
		t.Fatal("Failed to persist test data")
	}
	cleanUpPersistedTestData(t)
	persistedDataPath = tempPath
}

func cleanUpPersistedTestData(t testing.TB) {
	if persistedDataPath != "" {
		if err := os.RemoveAll(persistedDataPath); err != nil {
			t.Fatal(err)
		}
	}
	persistedDataPath = ""
}

func generateAuthorization(t testing.TB) string {
	app := newPersistentTestApp(t)
	defer app.Cleanup()

	organizerCName := CName[TournamentOrganizer]()

	record := &core.Record{}
	err := app.RecordQuery(organizerCName).Limit(1).One(record)
	if err != nil {
		t.Fatal(err)
	}

	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	return token
}

func authHeader(token string) map[string]string {
	return map[string]string{
		"Authorization": token,
	}
}

func recordToRequestBody(t testing.TB, record core.RecordProxy) io.Reader {
	rawJson, err := record.ProxyRecord().MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(rawJson)
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
