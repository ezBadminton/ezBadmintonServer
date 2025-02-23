package api

import (
	"fmt"
	"net/http"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

func proxyId[P Proxy, PP ProxyP[P]](pathValueName string) *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			id := e.Request.PathValue(pathValueName)
			proxy, err := store.FindProxy[P, PP](id)
			if err != nil {
				errMsg := fmt.Sprintf("could not find the %v ID", pathValueName)
				return e.String(http.StatusBadRequest, errMsg)
			}

			e.Set(pathValueName, proxy)

			return e.Next()
		},
	}
}
