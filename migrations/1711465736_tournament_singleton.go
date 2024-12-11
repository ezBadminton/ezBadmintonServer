package migrations

import (
	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		tournamentCollection, err := app.FindCollectionByNameOrId(names.Collections.Tournaments)
		if err != nil {
			return err
		}

		tournament := core.NewRecord(tournamentCollection)
		tournament.Set(names.Fields.Tournaments.Title, "The Tournament")
		tournament.Set(names.Fields.Tournaments.DontReprintGameSheets, true)
		tournament.Set(names.Fields.Tournaments.PrintQrCodes, true)
		tournament.Set(names.Fields.Tournaments.PlayerRestTime, 20)
		tournament.Set(names.Fields.Tournaments.QueueMode, "manual")

		return app.Save(tournament)
	}, func(app core.App) error {
		return nil
	})
}
