package api

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

var BuildVersion string = "dev"

func BindVersionApiHooks(app core.App) {
	url := "/api/version"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := e.Router.Group(url)

		group.GET("", sendVersion)

		return e.Next()
	})
}

func sendVersion(e *core.RequestEvent) error {
	return e.String(http.StatusOK, BuildVersion)
}
