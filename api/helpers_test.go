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
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/apis"
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

// Returns a test app instance like newPersistentTestApp but additionally
// triggers the serve event. For use without the ApiScenario.
func newPersistentTestAppAndServe(t testing.TB) *tests.TestApp {
	app := newPersistentTestApp(t)

	baseRouter, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}

	serveEvent := new(core.ServeEvent)
	serveEvent.App = app
	serveEvent.Router = baseRouter

	err = app.OnServe().Trigger(serveEvent)
	if err != nil {
		t.Fatal(err)
	}

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

type testCompetitionSettings struct {
	tType              TournamentType
	numTeams, teamSize int
}

// Creates players, puts them into teams, registers them to a newly
// created competition, sets the tournement mode, makes a draw
// and starts the tournament
// Returns the competition, the teams, the players and the tournament plan.
func (_ commonTestScenarios) setUpTestTournament(
	settings testCompetitionSettings,
	t *testing.T, headers map[string]string,
) (*Competition, []*Team, []*Player, map[string]any) {
	numPlayers := settings.numTeams * settings.teamSize
	players := createTestPlayers(t, numPlayers, Attending)
	competition := createTestCompetition(t, settings.teamSize, Any)
	registrationURL := fmt.Sprintf("/api/ezbadminton/admin/registration/%v", competition.Id)
	drawURL := fmt.Sprintf("/api/ezbadminton/admin/draw/%v/make", competition.Id)
	startURL := fmt.Sprintf("/api/ezbadminton/admin/tournaments/%v/start", competition.Id)

	for i := range settings.numTeams {
		playerIds := make([]string, settings.teamSize)
		basePlayerIndex := i * settings.teamSize
		for playerI := range settings.teamSize {
			player := players[basePlayerIndex+playerI]
			playerIds[playerI] = player.Id
		}

		body := map[string]any{
			"players": playerIds,
		}

		rawJson, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		json := bytes.NewReader(rawJson)

		registrationScenario := tests.ApiScenario{
			Name:           "register a team for a general competition setup",
			Method:         http.MethodPost,
			URL:            registrationURL,
			Headers:        headers,
			Body:           json,
			ExpectedStatus: 200,
			TestAppFactory: newPersistentTestApp,
			AfterTestFunc:  persistTestData,
		}

		registrationScenario.Test(t)
	}

	competitionCName := CName[Competition]()
	teamIds := make([]string, 0)
	registrationFetchScenario := tests.ApiScenario{
		Name:            "fetch the registered team IDs of a general competition setup",
		Method:          http.MethodGet,
		URL:             fmt.Sprintf("/api/collections/%v/records/%v", competitionCName, competition.Id),
		Headers:         headers,
		ExpectedStatus:  200,
		TestAppFactory:  newPersistentTestApp,
		ExpectedContent: []string{"registrations"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			response := unmarshalListRespose(res)
			ids := response["registrations"].([]any)
			for _, id := range ids {
				teamIds = append(teamIds, id.(string))
			}
		},
	}
	registrationFetchScenario.Test(t)

	app := newPersistentTestApp(t)
	defer app.Cleanup()

	teams := make([]*Team, 0)
	for _, id := range teamIds {
		team, err := store.FindProxy[Team](id)
		if err != nil {
			t.Fatal(err)
		}
		teams = append(teams, team)
	}

	modeSettings, _ := NewProxy[TournamentModeSettings](app)
	modeSettings.SetType(settings.tType)
	modeSettings.SetSeedingMode(TieredSeeds)
	modeSettings.SetWinningPoints(21)
	modeSettings.SetWinningSets(2)
	modeSettings.SetMaxPoints(30)
	modeSettings.SetTwoPointMargin(true)
	modeSettings.SetRaw("competitions", []string{competition.Id})
	modeSettings.WithCustomData(true)

	modeSettingsCName := CName[TournamentModeSettings]()
	modeSettingsScenario := tests.ApiScenario{
		Name:            "set tournament mode settings for general competition setup",
		Method:          http.MethodPost,
		URL:             fmt.Sprintf("/api/collections/%v/records", modeSettingsCName),
		Headers:         headers,
		Body:            recordToRequestBody(t, modeSettings),
		ExpectedStatus:  200,
		ExpectedContent: []string{modeSettingsCName},
		TestAppFactory:  newPersistentTestApp,
		AfterTestFunc:   persistTestData,
	}
	modeSettingsScenario.Test(t)

	drawScenario := tests.ApiScenario{
		Name:           "make draw for general competition setup",
		Method:         http.MethodPost,
		URL:            drawURL,
		Headers:        headers,
		ExpectedStatus: 200,
		TestAppFactory: newPersistentTestApp,
		AfterTestFunc:  persistTestData,
	}
	drawScenario.Test(t)

	startScenario := tests.ApiScenario{
		Name:           "start competition for general competition setup",
		Method:         http.MethodPost,
		URL:            startURL,
		Headers:        headers,
		ExpectedStatus: 200,
		TestAppFactory: newPersistentTestApp,
		AfterTestFunc:  persistTestData,
	}
	startScenario.Test(t)

	var plan map[string]any
	tournamentFetchScenario := tests.ApiScenario{
		Name:            "fetch tournament plan of general competition setup",
		Method:          http.MethodGet,
		URL:             "/api/collections/tournament_plans/records",
		Headers:         headers,
		ExpectedStatus:  200,
		ExpectedContent: []string{"items"},
		TestAppFactory:  newPersistentTestApp,
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			response := unmarshalListRespose(res)
			plans := response["items"].([]any)
			for _, p := range plans {
				p := p.(map[string]any)
				if p["competition"].(string) == competition.Id {
					plan = p
				}
			}
		},
	}
	tournamentFetchScenario.Test(t)

	return competition, teams, players, plan
}

func setUpCourts(t testing.TB, amount int) []*Court {
	app := newPersistentTestAppAndServe(t)
	defer app.Cleanup()

	gym, _ := NewProxy[Gymnasium](app)
	if err := app.Save(gym); err != nil {
		t.Fatal(err)
	}

	for i := range amount {
		court, _ := NewProxy[Court](app)
		court.SetGymnasium(gym)
		court.SetName(fmt.Sprintf("Court %v", i))
		if err := app.Save(court); err != nil {
			t.Fatal(err)
		}
	}
	persistTestData(t, app, nil)

	courtStore, _ := store.FindRecordStore[Court]()
	courts := courtStore.RecordList

	return courts
}

func fetchSingleEliminationRounds(t testing.TB, tPlan map[string]any) [][]*MatchData {
	tournament := tPlan["tournament"].(map[string]any)
	tType := tournament["type"].(string)
	if tType != "SingleElimination" {
		t.Fatal("the given tournament plan is not of a single elimination tournament")
	}
	rounds := tournament["rounds"].([]any)
	result := make([][]*MatchData, 0)
	for _, round := range rounds {
		round := round.([]any)
		roundData := make([]*MatchData, 0)
		for _, matchId := range round {
			matchId := matchId.(string)
			matchData, err := store.FindProxy[MatchData](matchId)
			if err != nil {
				t.Fatal(err)
			}
			roundData = append(roundData, matchData)
		}
		result = append(result, roundData)
	}
	return result
}

func init() {
	commonScenarios = commonTestScenarios{}
}
