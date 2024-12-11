package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// FetchCollection returns all Records of a collection
func FetchCollection(collectionName string, dao core.App) ([]*core.Record, error) {
	collectionFetch := dao.RecordQuery(collectionName)
	records := []*core.Record{}
	if err := collectionFetch.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}

func FetchAndExpandCollection(collectionName string, dao core.App) ([]*core.Record, error) {
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
func GetSingle(records []*core.Record, fieldName string) *core.Record {
	var singleOne *core.Record = nil

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

// Executes the given function on every given record model.
// If an error is encountered it is returned and the remaining records wont be processed.
func ProcessAsModels(models []*core.Record, fn func(model core.Model) error) error {
	for _, model := range models {
		if err := fn(model); err != nil {
			return err
		}
	}

	return nil
}

// Executes the given function on every given record.
// If an error is encountered it is returned and the remaining records wont be processed.
func ProcessAsRecords(records []*core.Record, fn func(record *core.Record) error) error {
	for _, record := range records {
		if err := fn(record); err != nil {
			return err
		}
	}

	return nil
}

func DeleteModelsById(collectionName string, ids []string, dao core.App) error {
	idArray := make([]interface{}, len(ids))
	for i := range ids {
		idArray[i] = ids[i]
	}

	models := make([]*core.Record, 0, len(ids))

	return dao.RunInTransaction(func(txDao core.App) error {
		if err := txDao.RecordQuery(collectionName).AndWhere(dbx.In("id", idArray...)).All(&models); err != nil {
			return err
		}

		if err := ProcessAsModels(models, txDao.Delete); err != nil {
			return err
		}

		return nil
	})
}
