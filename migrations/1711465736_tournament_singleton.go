package migrations

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		cName := CName[TournamentEvent]()
		tournamentCollection, err := app.FindCollectionByNameOrId(cName)
		if err != nil {
			return err
		}

		tournament := TournamentEvent{}
		tournament.SetProxyRecord(core.NewRecord(tournamentCollection))
		tournament.Id = "snonkychallenge"
		tournament.SetTitle("TheTournament")
		tournament.SetDontReprintGameSheets(true)
		tournament.SetPrintQrCodes(true)
		tournament.SetPlayerRestTime(10)
		tournament.SetQueueMode(Manual)
		tournament.SetRaw("created", types.NowDateTime())
		tournament.SetRaw("updated", types.NowDateTime())

		return app.Save(tournament)
	}, func(app core.App) error {
		return nil
	})
}
