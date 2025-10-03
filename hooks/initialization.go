package hooks

import (
	"github.com/ezBadminton/ezBadmintonServer/api"
	"github.com/ezBadminton/ezBadmintonServer/infoscreens"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func InitHooksAndApi(app core.App) {
	api.InitRootApiRoute(app)

	api.BindOrganizerRegistrationHooks(app)
	api.BindClubHooks(app)
	api.BindRegistrationHooks(app)
	api.BindTournamentPlanHooks(app)
	api.BindTournamentMatchHooks(app)
	api.BindDrawHooks(app)
	api.BindPlayerStatusHooks(app)
	api.BindCourtHooks(app)
	api.BindMatchHooks(app)
	api.BindStartStopHooks(app)
	api.BindTieBreakerHooks(app)
	api.BindQualificationOverrideHooks(app)
	api.BindMatchScheduleHooks(app)
	api.BindEventSettingsHooks(app)
	api.BindCategoryHooks(app)
	api.BindTournamentModeSettingsHooks(app)
	api.BindCompetitionHooks(app)
	api.BindInfoScreenControlHooks(app)
	api.BindVersionApiHooks(app)
	api.BindStartingFeeHooks(app)
	api.BindDevTestingHooks(app)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := store.InitStores(e.App); err != nil {
			return err
		}
		tops.InitTournamentOperations(e.App)
		infoscreens.InitTokenManager(app)

		return e.Next()
	})

	app.Cron().Add("organizer_ping", "*/4 * * * *", func() {
		tops.PingOrganizerClients(app)
	})
}
