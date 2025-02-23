package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

var rootGroup *router.RouterGroup[*core.RequestEvent]

func InitRootApiRoute(app core.App) {
	url := "/api/ezbadminton/admin"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		rootGroup = e.Router.Group(url)

		organizerCName := CName[TournamentOrganizer]()
		rootGroup.Bind(apis.RequireAuth(organizerCName))

		return e.Next()
	})
}
