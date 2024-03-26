package main

import (
	names "github.com/ezBadminton/ezBadmintonServer/schema_names"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// HandleBeforeGymnasiumDelete deletes the courts of a gymnasium before the gymnasium is deleted
func HandleBeforeGymnasiumDelete(deletedGym *models.Record, dao *daos.Dao) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		courtsOfGym := make([]*models.Record, 0, 6)

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

		if err := DeleteRecordsById(names.Collections.Courts, courtIds, txDao); err != nil {
			return err
		}

		return nil
	})
}
