package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

func bulkSetPlayerStatus(app core.App, players []*Player, status PlayerStatus) error {
	return app.RunInTransaction(func(txApp core.App) error {
		for _, player := range players {
			player.SetStatus(status)
			player.Set("testUnit", "Ttttest")
			player.WithCustomData(true)
			if err := txApp.Save(player); err != nil {
				return err
			}
		}
		return nil
	})
}
