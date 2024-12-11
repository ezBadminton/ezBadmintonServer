package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		adminCollection, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
		if err != nil {
			return err
		}

		admin := core.NewRecord(adminCollection)
		admin.SetEmail("test@example.com")
		admin.SetPassword("1234567890")

		return app.Save(admin)
	}, func(app core.App) error {
		return nil
	})
}
