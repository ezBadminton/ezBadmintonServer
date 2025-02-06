package collection

import (
	"errors"
	"slices"

	"github.com/pocketbase/pocketbase/core"
)

type RelationStore struct {
	// The reverse relations store all records that a given record
	// is related to as the relation child
	// record ID -> parent record -> field names
	reverseRelations map[string]map[*core.Record][]string
}

func newRelationStore() *RelationStore {
	return &RelationStore{
		reverseRelations: make(map[string]map[*core.Record][]string),
	}
}

func (r *RelationStore) ExpandRelations(collectionName string, records ...core.RecordProxy) error {
	relations := Relations[collectionName]

	for _, record := range records {
		r.resetReverseRelations(record.ProxyRecord())
	}

	for relatedCollectionName, relatedFields := range relations {
		relatedStore, err := FindRecordStore(relatedCollectionName)
		if err != nil {
			return err
		}

		if err := r.expandRecordRelations(records, relatedStore, relatedFields); err != nil {
			return err
		}
	}
	return nil
}

// Remove the record from all its relation parents
func (r *RelationStore) RemoveFromRelations(record *core.Record) {
	r.resetReverseRelations(record)

	relations := r.reverseRelations[record.Id]
	for parent, fields := range relations {
		relationFields := parent.Expand()

		for _, field := range fields {
			switch relation := relationFields[field].(type) {
			case []*core.Record:
				relationFields[field] = slices.DeleteFunc(
					relation,
					func(r *core.Record) bool { return r == record },
				)
			case *core.Record:
				delete(relationFields, field)
			}
		}

		parent.SetExpand(relationFields)
	}
}

func (r *RelationStore) resetReverseRelations(record *core.Record) {
	for _, reverseRelations := range r.reverseRelations {
		for parent := range reverseRelations {
			if parent.Id == record.Id {
				delete(reverseRelations, parent)
			}
		}
	}
}

func (r *RelationStore) expandRecordRelations(records []core.RecordProxy, relatedStore RecordStore, relatedFields []relationField) error {
	for _, record := range records {
		proxyRelations := make(map[string]any)

		for _, relatedField := range relatedFields {
			var err error
			if relatedField.isMulti {
				err = r.expandMultiRelation(record, proxyRelations, relatedStore, relatedField.fieldName)
			} else {
				err = r.expandSingleRelation(record, proxyRelations, relatedStore, relatedField.fieldName)
			}
			if err != nil {
				return err
			}
		}

		record.ProxyRecord().SetExpand(proxyRelations)
	}
	return nil
}

func (r *RelationStore) expandSingleRelation(record core.RecordProxy, proxyRelations map[string]any, relatedStore RecordStore, relatedFieldName string) error {
	pRecord := record.ProxyRecord()
	relatedId := pRecord.GetString(relatedFieldName)
	if relatedId == "" {
		return nil
	}

	relatedRecord, ok := relatedStore.FindRecord(relatedId)
	if !ok {
		return errors.New("the related record is not in the store")
	}

	relatedPRecord := relatedRecord.ProxyRecord()
	proxyRelations[relatedFieldName] = relatedPRecord
	r.storeReverseRelation(pRecord, relatedPRecord, relatedFieldName)

	return nil
}

func (r *RelationStore) expandMultiRelation(record core.RecordProxy, proxyRelations map[string]any, relatedStore RecordStore, relatedFieldName string) error {
	pRecord := record.ProxyRecord()
	relatedIds := pRecord.GetStringSlice(relatedFieldName)
	relatedRecords := make([]*core.Record, len(relatedIds))

	for i, id := range relatedIds {
		relatedRecord, ok := relatedStore.FindRecord(id)
		if !ok {
			return errors.New("the related record is not in the store")
		}

		relatedPRecord := relatedRecord.ProxyRecord()
		relatedRecords[i] = relatedPRecord
		r.storeReverseRelation(pRecord, relatedPRecord, relatedFieldName)
	}

	proxyRelations[relatedFieldName] = relatedRecords

	return nil
}

func (r *RelationStore) storeReverseRelation(record, relation *core.Record, relatedFieldName string) {
	id := relation.Id
	_, ok := r.reverseRelations[id]
	if !ok {
		r.reverseRelations[id] = make(map[*core.Record][]string)
	}
	_, ok = r.reverseRelations[id][record]
	if !ok {
		r.reverseRelations[id][record] = make([]string, 0)
	}

	r.reverseRelations[id][record] = append(r.reverseRelations[id][record], relatedFieldName)
}
