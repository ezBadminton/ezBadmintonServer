package api

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

func BindPlayerHooks(app core.App) {
	cName := CName[Player]()

	app.OnRecordCreate(cName).BindFunc(OnPlayerWithNewClub)
	app.OnRecordUpdate(cName).BindFunc(OnPlayerWithNewClub)

	app.OnRecordAfterUpdateSuccess(cName).BindFunc(OnPlayerClubChange)
	app.OnRecordAfterDeleteSuccess(cName).BindFunc(OnPlayerClubChange)
}

// Checks if the player comes with a newly created club
// and persist that club in the same transaction as the player.
func OnPlayerWithNewClub(e *core.RecordEvent) error {
	newClubId := e.Record.GetString("club")
	newClubName := newClubName(newClubId)

	if newClubName == "" {
		return e.Next()
	}

	app := e.App
	err := app.RunInTransaction(func(txApp core.App) error {
		e.App = txApp

		newClubId, err := saveNewClub(newClubName, e.App)
		if err != nil {
			return err
		}
		e.Record.Set("club", newClubId)

		return e.Next()
	})
	e.App = app

	return err
}

// Checks if the update/delete of a player caused the
// player's club to be empty and deletes the club.
func OnPlayerClubChange(e *core.RecordEvent) error {
	oldPlayer, err := store.FindProxy[Player](e.Record.Id)
	if err != nil {
		return err
	}
	oldClub := oldPlayer.Club()

	var newClubId string
	if e.Type == core.ModelEventTypeUpdate {
		newClubId = e.Record.GetString("club")
	}

	deleteOldClub := false
	if oldClub != nil && oldClub.Id != newClubId {
		playersInOldClub := store.ListRelationParents(oldClub.Record)
		deleteOldClub = len(playersInOldClub) == 1
	}

	if deleteOldClub {
		if err := e.App.Delete(oldClub); err != nil {
			return err
		}
	}

	return e.Next()
}

func saveNewClub(name string, app core.App) (string, error) {
	newClub, err := NewProxy[Club](app)
	if err != nil {
		return "", err
	}
	newClub.SetName(name)

	if err := app.Save(newClub); err != nil {
		return "", err
	}

	return newClub.Id, nil
}

// Returns the name of a new club if the newClubId transports one
func newClubName(newClubId string) string {
	prefix := "newclub:"
	name := ""
	if len(newClubId) > len(prefix) && newClubId[:len(prefix)] == prefix {
		name = newClubId[len(prefix):]
	}
	return name
}
