package api_test

import (
	"bytes"
	"encoding/json"
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

func unmarshalListRespose(res *http.Response) map[string]any {
	buf := bytes.Buffer{}
	buf.ReadFrom(res.Body)
	responseJson := map[string]any{}
	json.Unmarshal(buf.Bytes(), &responseJson)
	return responseJson
}

func unmarshalRecords[P Proxy, PP ProxyP[P]](app *tests.TestApp, res *http.Response) []PP {
	responseJson := unmarshalListRespose(res)
	items := responseJson["items"].([]any)

	cName := CName[P, PP]()
	collection, _ := app.FindCachedCollectionByNameOrId(cName)
	records := make([]PP, 0, len(items))
	for _, item := range items {
		record := core.NewRecord(collection)
		recordData := item.(map[string]any)
		record.Load(recordData)
		pRecord, _ := WrapRecord[P, PP](record)
		records = append(records, pRecord)
	}
	return records
}

var commonScenarios commonTestScenarios

type commonTestScenarios struct{}

func (_ commonTestScenarios) registerOrganizer() *tests.ApiScenario {
	organizerCName := CName[TournamentOrganizer]()
	return &tests.ApiScenario{
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
	}
}

func (_ commonTestScenarios) registerTeam(headers map[string]string, competition *Competition, players []*Player) *tests.ApiScenario {
	sb := strings.Builder{}
	for i, player := range players {
		sb.WriteRune('"')
		sb.WriteString(player.Id)
		sb.WriteRune('"')
		if i < len(players)-1 {
			sb.WriteRune(',')
		}
	}
	playerIdList := sb.String()
	return &tests.ApiScenario{
		Name:    "register a team",
		Method:  http.MethodPost,
		URL:     fmt.Sprintf("/api/ezbadminton/admin/registration/%v", competition.Id),
		Headers: headers,
		Body: strings.NewReader(fmt.Sprintf(`{
				"players": [%v]
			}`, playerIdList)),
		ExpectedStatus: 200,
		TestAppFactory: newPersistentTestApp,
		AfterTestFunc:  persistTestData,
	}
}

func init() {
	commonScenarios = commonTestScenarios{}
}
