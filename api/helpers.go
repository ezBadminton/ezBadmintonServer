package api

import (
	"errors"
	"net/http"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

func idList[S ~[]P, P core.RecordProxy](records S) []string {
	ids := make([]string, len(records))
	for i, r := range records {
		ids[i] = r.ProxyRecord().Id
	}
	return ids
}

func findCompetition(r *http.Request) (*Competition, error) {
	competitionId := r.PathValue("competition")
	competition, err := store.FindProxy[Competition](competitionId)
	if err != nil {
		return nil, errors.New("could not find the competition ID")
	}
	return competition, nil
}

func idFinder[P Proxy, PP ProxyP[P]](id string) func(p PP) bool {
	return func(p PP) bool {
		return p.ProxyRecord().Id == id
	}
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

func oldNew[P Proxy, PP ProxyP[P]](e *core.RecordEvent) (PP, PP, error) {
	old, err := store.FindProxy[P, PP](e.Record.Id)
	if err != nil {
		return nil, nil, err
	}

	new, _ := WrapRecord[P, PP](e.Record)
	if err := store.ExpandRelationsDry(new); err != nil {
		return nil, nil, err
	}

	return old, new, nil
}
