package migrations

import (
	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		tournamentCollection, err := dao.FindCollectionByNameOrId(names.Collections.Tournaments)
		if err != nil {
			return err
		}

		tournament := models.NewRecord(tournamentCollection)
		tournament.Set(names.Fields.Tournaments.Title, "The Tournament")
		tournament.Set(names.Fields.Tournaments.DontReprintGameSheets, true)
		tournament.Set(names.Fields.Tournaments.PrintQrCodes, true)
		tournament.Set(names.Fields.Tournaments.PlayerRestTime, 20)
		tournament.Set(names.Fields.Tournaments.QueueMode, "manual")

		return dao.SaveRecord(tournament)
	}, func(db dbx.Builder) error {
		return nil
	})
}
