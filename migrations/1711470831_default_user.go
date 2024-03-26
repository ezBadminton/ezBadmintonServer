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

		collection, err := dao.FindCollectionByNameOrId(names.Collections.TournamentOrganizer)
		if err != nil {
			return err
		}

		record := models.NewRecord(collection)
		record.SetUsername("testuser")
		record.SetPassword("12345")

		return dao.SaveRecord(record)
	}, func(db dbx.Builder) error {
		return nil
	})
}
