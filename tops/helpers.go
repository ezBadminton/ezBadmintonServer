package tops

import (
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

// These keys are used for custom data entries
// in records that serve for passing data down
// the hook chain
const (
	RegistrationCreateKey = "CREATED_REGISTRATION"
	RegistrationUpdateKey = "UPDATED_REGISTRATION"
	RegistrationDeleteKey = "DELETED_REGISTRATION"
)

func idFinder[P Proxy, PP ProxyP[P]](id string) func(p PP) bool {
	return func(p PP) bool {
		return p.ProxyRecord().Id == id
	}
}

func idList[S ~[]P, P core.RecordProxy](records S) []string {
	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.ProxyRecord().Id
	}
	return ids
}

// Returns true if a contains all proxies of b compared by ID.
func containsAll[S ~[]PP, P Proxy, PP ProxyP[P]](a, b S) bool {
	for _, p := range b {
		if !slices.ContainsFunc(a, idFinder[P, PP](p.ProxyRecord().Id)) {
			return false
		}
	}
	return true
}

func matchesToMatchData(matches []*got.Match) []*MatchData {
	matchData := make([]*MatchData, len(matches))
	for i, m := range matches {
		matchData[i] = Tournaments.matchData[m.Id()]
	}
	return matchData
}

func realtimeActionAndData[T any, P *T](createKey, updateKey, deleteKey string, data map[string]any) (string, P) {
	keys := []string{createKey, updateKey, deleteKey}
	actions := []string{core.ModelEventTypeCreate, core.ModelEventTypeUpdate, core.ModelEventTypeDelete}

	var r any
	var ok bool
	var action string
	for i, key := range keys {
		if r, ok = data[key]; ok {
			action = actions[i]
			break
		}
	}

	if action == "" {
		return "", nil
	}

	record := r.(P)

	return action, record
}
