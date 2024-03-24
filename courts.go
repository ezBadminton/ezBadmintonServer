package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// HandleBeforeGymnasiumDelete deletes the courts of a gymnasium before the gymnasium is deleted
func HandleBeforeGymnasiumDelete(deletedGym *models.Record, dao *daos.Dao) error {
	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		courtsOfGym := make([]*models.Record, 0, 6)

		err := txDao.
			RecordQuery(courtsName).
			AndWhere(dbx.HashExp{courtGymnasiumName: deletedGym.Id}).
			All(&courtsOfGym)

		if err != nil {
			return err
		}

		courtIds := make([]string, 0, len(courtsOfGym))
		for _, court := range courtsOfGym {
			courtIds = append(courtIds, court.Id)
		}

		if err := DeleteRecordsById(courtsName, courtIds, txDao); err != nil {
			return err
		}

		return nil
	})
}
