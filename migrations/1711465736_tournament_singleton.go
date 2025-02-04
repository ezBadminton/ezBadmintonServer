package migrations

import (
	"github.com/ezBadminton/ezBadmintonServer/collection"
	. "github.com/ezBadminton/ezBadmintonServer/generated"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		cName := collection.Names[collection.Tournaments]
		tournamentCollection, err := app.FindCollectionByNameOrId(cName)
		if err != nil {
			return err
		}

		tournament := Tournament{}
		tournament.SetProxyRecord(core.NewRecord(tournamentCollection))
		tournament.SetTitle("TheTournament")
		tournament.SetDontReprintGameSheets(true)
		tournament.SetPrintQrCodes(true)
		tournament.SetPlayerRestTime(20)
		tournament.SetQueueMode(Manual)

		return app.Save(tournament)
	}, func(app core.App) error {
		return nil
	})
}
