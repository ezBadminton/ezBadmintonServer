package api

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func BindResultHooks(app core.App) {
	url := "/api/ezbadminton/result/{competition}"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)
		group.Bind(apis.RequireAuth())

		group.POST("", postResult)

		return e.Next()
	})
}

func postResult(e *core.RequestEvent) error {
	// Needs the scheduler to determine running matches
	return e.NoContent(http.StatusOK)
}
