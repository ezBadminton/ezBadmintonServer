package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// FetchCollection returns all Records of a collection
func FetchCollection(collectionName string, dao *daos.Dao) ([]*models.Record, error) {
	collectionFetch := dao.RecordQuery(collectionName)
	records := []*models.Record{}
	if err := collectionFetch.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}

func FetchAndExpandCollection(collectionName string, dao *daos.Dao) ([]*models.Record, error) {
	competitions, fetchErr := FetchCollection(collectionName, dao)
	if fetchErr != nil {
		return nil, fetchErr
	}

	if err := ExpandAllNestedRelations(competitions, dao); err != nil {
		return nil, err
	}

	return competitions, nil
}

// Returns the single record that has a non-empty value in the field with fieldName.
// If there is none or more than one then nil is returned.
func GetSingle(records []*models.Record, fieldName string) *models.Record {
	var singleOne *models.Record = nil

	for _, record := range records {
		if record.ExpandedOne(fieldName) != nil {
			if singleOne != nil {
				return nil
			} else {
				singleOne = record
			}
		}
	}

	return singleOne
}

// Executes the given function on every given record.
// If an error is encountered it is returned and the remaining records wont be processed.
func ProcessRecords(records []*models.Record, fn func(record *models.Record) error) error {
	for _, record := range records {
		if err := fn(record); err != nil {
			return err
		}
	}

	return nil
}

func DeleteRecordsById(collectionName string, ids []string, dao *daos.Dao) error {
	idArray := make([]interface{}, len(ids))
	for i := range ids {
		idArray[i] = ids[i]
	}

	records := make([]*models.Record, 0, len(ids))

	return dao.RunInTransaction(func(txDao *daos.Dao) error {
		if err := txDao.RecordQuery(collectionName).AndWhere(dbx.In("id", idArray...)).All(&records); err != nil {
			return err
		}

		if err := ProcessRecords(records, txDao.DeleteRecord); err != nil {
			return err
		}

		return nil
	})
}
