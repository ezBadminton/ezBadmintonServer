package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
)

// Expands all relations that the schema of the records has
//
// All records have to have the same schema e.g. be from the same collection
func ExpandAllRelations(records []*models.Record, dao *daos.Dao) error {
	if len(records) == 0 {
		return nil
	}
	var fields []*schema.SchemaField = records[0].Collection().Schema.Fields()

	relationFields := make([]string, 0, len(fields))

	for _, field := range fields {
		if field.Type == "relation" {
			relationFields = append(relationFields, field.Name)
		}
	}

	var error map[string]error = dao.ExpandRecords(records, relationFields, nil)

	if len(error) > 0 {
		return fmt.Errorf("expansion of all relation fields failed:\n%v", error)
	}

	return nil
}

// Expands all relations according to the schema of the records and
// recursively does the same for the expanded relations
func ExpandAllNestedRelations(records []*models.Record, dao *daos.Dao) error {
	if len(records) == 0 {
		return nil
	}

	if err := ExpandAllRelations(records, dao); err != nil {
		return err
	}

	relatedRecords := getAllUnexpandedRelations(records)

	for _, records := range relatedRecords {
		if err := ExpandAllNestedRelations(records, dao); err != nil {
			return err
		}
	}

	return nil
}

// RegisterRelationUpdateCascade reigsters a hook that cascades update events of related models
// under the field name up to the models in the collection.
func RegisterRelationUpdateCascade(
	collectionName string,
	fieldName string,
	app *pocketbase.PocketBase,
) error {
	relationCollection, err := app.Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}

	var relationGetter func(string, string, string, *daos.Dao) ([]*models.Record, error)

	relationOptions := relationCollection.Schema.GetFieldByName(fieldName).Options.(*schema.RelationOptions)

	if relationOptions.IsMultiple() {
		relationGetter = FindReverseMultiRelations
	} else {
		relationGetter = FindReverseRelations
	}

	app.OnModelAfterUpdate(relationOptions.CollectionId).Add(func(e *core.ModelEvent) error {
		relations, err := relationGetter(e.Model.GetId(), collectionName, fieldName, app.Dao())
		if err != nil {
			return nil
		}

		if err := CascadeRelationUpdate(relations, app.Dao()); err != nil {
			return err
		}

		return nil
	})

	return nil
}

// Returns all the records that are related to the given records mapped by their collection.
// The slices contain no duplicates even if the same record is in multiple relations.
// Also the returned relations will not include records which already have their own relations
// expanded to avoid circular expansions.
func getAllUnexpandedRelations(records []*models.Record) map[string][]*models.Record {
	allRelationsSet := make(map[*models.Record]struct{})

	for _, record := range records {
		relations := make([]*models.Record, 0, 3)

		for _, relation := range record.Expand() {
			switch r := relation.(type) {
			case *models.Record:
				relations = append(relations, r)
			case []*models.Record:
				relations = append(relations, r...)
			}
		}

		for _, relatedRecord := range relations {
			if len(relatedRecord.Expand()) == 0 {
				allRelationsSet[relatedRecord] = struct{}{}
			}
		}
	}

	allRelations := make(map[string][]*models.Record)
	for relation := range allRelationsSet {
		_, exists := allRelations[relation.Collection().Name]
		if !exists {
			allRelations[relation.Collection().Name] = make([]*models.Record, 0)
		}

		allRelations[relation.Collection().Name] = append(allRelations[relation.Collection().Name], relation)
	}

	return allRelations

}

// FindReverseMultiRelations queries the DB for records from the relationCollection that have
// the relationId as their relation under the relationFieldName.
func FindReverseRelations(
	relationId string,
	relationCollectionName string,
	relationFieldName string,
	dao *daos.Dao,
) ([]*models.Record, error) {
	var relatedRecords []*models.Record = make([]*models.Record, 0, 1)

	err := dao.RecordQuery(relationCollectionName).
		AndWhere(dbx.HashExp{relationFieldName: relationId}).
		All(&relatedRecords)

	if err != nil {
		return nil, err
	}

	return relatedRecords, nil
}

// FindReverseMultiRelations queries the DB for records from the relationCollection that have
// the relationId in their multi-relation under the relationFieldName.
func FindReverseMultiRelations(
	relationId string,
	relationCollectionName string,
	relationFieldName string,
	dao *daos.Dao,
) ([]*models.Record, error) {
	var relatedRecords []*models.Record = make([]*models.Record, 0, 1)

	err := dao.RecordQuery(relationCollectionName).
		AndWhere(
			dbx.Exists(dbx.NewExp(
				fmt.Sprintf("SELECT 1 FROM json_each(%s) WHERE value={:id}", relationFieldName),
				dbx.Params{"id": relationId},
			)),
		).
		All(&relatedRecords)

	if err != nil {
		return nil, err
	}

	return relatedRecords, nil
}

// CascadeUpdate updated the relatedRecords without changing their data. These updates
// trigger the realtime API update event to notify clients of the change in their
// relation. Thus the update of the relation is cascaded to the related records.
func CascadeRelationUpdate(
	relatedRecords []*models.Record,
	dao *daos.Dao,
) error {
	if len(relatedRecords) == 0 {
		return nil
	}

	if err := ProcessRecords(relatedRecords, dao.SaveRecord); err != nil {
		return err
	}

	return nil
}
