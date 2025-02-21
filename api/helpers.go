package api

import (
	"errors"
	"fmt"
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

func findPathId[P Proxy, PP ProxyP[P]](pathValueName string, r *http.Request) (PP, error) {
	id := r.PathValue(pathValueName)
	proxy, err := store.FindProxy[P, PP](id)
	if err != nil {
		errMsg := fmt.Sprintf("could not find the %v ID", pathValueName)
		return nil, errors.New(errMsg)
	}
	return proxy, nil
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

func oldNew[P Proxy, PP ProxyP[P]](e *core.RecordEvent, expandDry bool) (PP, PP, error) {
	old, err := store.FindProxy[P, PP](e.Record.Id)
	if err != nil {
		return nil, nil, err
	}

	new, _ := WrapRecord[P, PP](e.Record)
	if expandDry {
		if err := store.ExpandRelationsDry(new); err != nil {
			return nil, nil, err
		}
	}

	return old, new, nil
}
