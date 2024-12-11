package main

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Expands all relations that the schema of the records has
//
// All records have to have the same schema e.g. be from the same collection
func ExpandAllRelations(records []*core.Record, dao core.App) error {
	if len(records) == 0 {
		return nil
	}
	var fields core.FieldsList = records[0].Collection().Fields

	relationFields := make([]string, 0, len(fields))

	for _, field := range fields {
		if _, ok := field.(*core.RelationField); ok {
			relationFields = append(relationFields, field.GetName())
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
func ExpandAllNestedRelations(records []*core.Record, dao core.App) error {
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
	relationCollection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}

	var relationGetter func(string, string, string, core.App) ([]*core.Record, error)

	relationField := relationCollection.Fields.GetByName(fieldName).(*core.RelationField)

	if relationField.IsMultiple() {
		relationGetter = FindReverseMultiRelations
	} else {
		relationGetter = FindReverseRelations
	}

	app.OnRecordUpdate(relationField.CollectionId).BindFunc(func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		relations, err := relationGetter(e.Record.Id, collectionName, fieldName, app)
		if err != nil {
			return err
		}

		if err := CascadeRelationUpdate(relations, app); err != nil {
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
func getAllUnexpandedRelations(records []*core.Record) map[string][]*core.Record {
	allRelationsSet := make(map[*core.Record]struct{})

	for _, record := range records {
		relations := make([]*core.Record, 0, 3)

		for _, relation := range record.Expand() {
			switch r := relation.(type) {
			case *core.Record:
				relations = append(relations, r)
			case []*core.Record:
				relations = append(relations, r...)
			}
		}

		for _, relatedRecord := range relations {
			if len(relatedRecord.Expand()) == 0 {
				allRelationsSet[relatedRecord] = struct{}{}
			}
		}
	}

	allRelations := make(map[string][]*core.Record)
	for relation := range allRelationsSet {
		_, exists := allRelations[relation.Collection().Name]
		if !exists {
			allRelations[relation.Collection().Name] = make([]*core.Record, 0)
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
	dao core.App,
) ([]*core.Record, error) {
	var relatedRecords []*core.Record = make([]*core.Record, 0, 1)

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
	dao core.App,
) ([]*core.Record, error) {
	var relatedRecords []*core.Record = make([]*core.Record, 0, 1)

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
	relatedRecords []*core.Record,
	dao core.App,
) error {
	if len(relatedRecords) == 0 {
		return nil
	}

	if err := ProcessAsModels(relatedRecords, dao.Save); err != nil {
		return err
	}

	return nil
}
