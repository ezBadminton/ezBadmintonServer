package main

import (
	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// HandleBeforeGymnasiumDelete deletes the courts of a gymnasium before the gymnasium is deleted
func HandleBeforeGymnasiumDelete(deletedGym *core.Record, dao core.App) error {
	return dao.RunInTransaction(func(txDao core.App) error {
		courtsOfGym := make([]*core.Record, 0, 6)

		err := txDao.
			RecordQuery(names.Collections.Courts).
			AndWhere(dbx.HashExp{names.Fields.Courts.Gymnasium: deletedGym.Id}).
			All(&courtsOfGym)

		if err != nil {
			return err
		}

		courtIds := make([]string, 0, len(courtsOfGym))
		for _, court := range courtsOfGym {
			courtIds = append(courtIds, court.Id)
		}

		if err := DeleteModelsById(names.Collections.Courts, courtIds, txDao); err != nil {
			return err
		}

		return nil
	})
}
