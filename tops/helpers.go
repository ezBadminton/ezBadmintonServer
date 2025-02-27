package tops

import (
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
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

func priorityHandler[R hook.Resolver](handler func(R) error, priority int) *hook.Handler[R] {
	return &hook.Handler[R]{
		Func:     handler,
		Priority: priority,
	}
}
