package api

import (
	"errors"
	"fmt"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

func pathId[P Proxy, PP ProxyP[P]](pathValueName string) *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			id := e.Request.PathValue(pathValueName)
			proxy, err := store.FindProxy[P, PP](id)
			if err != nil {
				errMsg := fmt.Sprintf("the %v ID does not exist", pathValueName)
				return e.BadRequestError(errMsg, err)
			}

			e.Set(pathValueName, proxy)

			return e.Next()
		},
	}
}

func bodyId[P Proxy, PP ProxyP[P]](bodyFieldName string) *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			proxies, err := readProxiesFromBody[P, PP](e, bodyFieldName)
			if err != nil {
				return e.BadRequestError(err.Error(), err)
			}

			e.Set(bodyFieldName, proxies[0])

			return e.Next()
		},
	}
}

func bodyIdList[P Proxy, PP ProxyP[P]](bodyFieldName string) *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Func: func(e *core.RequestEvent) error {
			proxies, err := readProxiesFromBody[P, PP](e, bodyFieldName)
			if err != nil {
				return e.BadRequestError(err.Error(), err)
			}

			e.Set(bodyFieldName, proxies)

			return e.Next()
		},
	}
}

func readProxiesFromBody[P Proxy, PP ProxyP[P]](e *core.RequestEvent, bodyFieldName string) ([]PP, error) {
	data := map[string]any{}
	e.BindBody(&data)

	var idList []string
	switch d := data[bodyFieldName].(type) {
	case string:
		idList = []string{d}
	case []any:
		if len(d) == 0 {
			return []PP{}, nil
		}
		ids, err := unpackStringList(d)
		if err != nil {
			errMsg := fmt.Sprintf("the '%v' field is not a list of IDs", bodyFieldName)
			return nil, errors.New(errMsg)
		}
		idList = ids
	default:
		errMsg := fmt.Sprintf("the body does not contain a '%v' ID field", bodyFieldName)
		return nil, errors.New(errMsg)
	}

	proxies := make([]PP, 0, len(idList))
	for _, id := range idList {
		p, err := store.FindProxy[P, PP](id)
		if err != nil {
			errMsg := fmt.Sprintf("the ID '%v' does not exist", id)
			return nil, errors.New(errMsg)
		}
		proxies = append(proxies, p)
	}

	return proxies, nil
}

func unpackStringList(list []any) ([]string, error) {
	stringList := make([]string, len(list))
	for i, e := range list {
		str, ok := e.(string)
		if !ok {
			return nil, errors.New("not a string list")
		}
		stringList[i] = str
	}
	return stringList, nil
}
