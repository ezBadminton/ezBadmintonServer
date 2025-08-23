package tops

import (
	"fmt"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

func createTestPlayers(app core.App, amount int) error {
	return app.RunInTransaction(func(txApp core.App) error {
		for i := 0; i < amount; i += 1 {
			lastName := fmt.Sprintf("%03d", i)
			firstName := fmt.Sprintf("Player-%s", lastName)

			player, err := NewProxy[Player](txApp)
			if err != nil {
				return err
			}
			player.SetFirstName(firstName)
			player.SetLastName(lastName)
			player.SetStatus(NotAttending)
			if err := txApp.Save(player); err != nil {
				return err
			}
		}
		return nil
	})
}
