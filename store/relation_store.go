package store

import (
	"errors"
	"maps"
	"slices"
	"sync"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
)

type RelationStore struct {
	// The reverse relations store all records that a given record
	// is related to as the relation child
	// record ID -> parent record -> field names
	reverseRelations map[string]map[*core.Record][]string
	mu               sync.Mutex
}

func newRelationStore() *RelationStore {
	return &RelationStore{
		reverseRelations: make(map[string]map[*core.Record][]string),
	}
}

func ExpandRelations[PP ProxyP[P], P Proxy](records ...PP) error {
	return expandRelations(true, records)
}

// Expands the relations but does not store the reverese relations
func ExpandRelationsDry[PP ProxyP[P], P Proxy](records ...PP) error {
	return expandRelations(false, records)
}

func expandRelations[PP ProxyP[P], P Proxy](storeReverse bool, records []PP) error {
	r := relationStore
	defer r.mu.Unlock()
	r.mu.Lock()

	collectionName := PP.CollectionName(nil)
	relations := Relations[collectionName]

	if storeReverse {
		for _, record := range records {
			r.resetReverseRelations(record.ProxyRecord())
		}
	}

	for relatedCollectionName, relatedFields := range relations {
		relatedStore, err := FindRecordStoreByCollectionName(relatedCollectionName)
		if err != nil {
			return err
		}

		if err := expandRecordRelations(records, relatedStore, relatedFields, storeReverse); err != nil {
			return err
		}
	}
	return nil
}

func ListRelationParents(record *core.Record) []*core.Record {
	r := relationStore
	r.mu.Lock()
	relMap, ok := r.reverseRelations[record.Id]
	r.mu.Unlock()

	if !ok {
		return nil
	}
	parents := make([]*core.Record, 0, len(relMap))
	for r := range relMap {
		parents = append(parents, r)
	}
	return parents
}

func RelationParentsByFieldName(record *core.Record) map[*core.Record][]string {
	r := relationStore
	defer r.mu.Unlock()
	r.mu.Lock()

	relMap, ok := r.reverseRelations[record.Id]
	if !ok {
		return nil
	}

	clone := make(map[*core.Record][]string, len(relMap))
	for k, v := range relMap {
		clone[k] = v
	}

	return clone
}

// Remove the record from all its relation parents
func (r *RelationStore) RemoveFromRelations(record *core.Record) {
	defer r.mu.Unlock()
	r.mu.Lock()

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

func expandRecordRelations[PP ProxyP[P], P Proxy](
	records []PP,
	relatedStore RecordStore,
	relatedFields []RelationField,
	storeReverse bool,
) error {
	r := relationStore
	for _, record := range records {
		proxyRelations := make(map[string]any)

		for _, relatedField := range relatedFields {
			var err error
			if relatedField.IsMulti {
				err = r.expandMultiRelation(record,
					proxyRelations,
					relatedStore,
					relatedField.FieldName,
					storeReverse,
				)
			} else {
				err = r.expandSingleRelation(record,
					proxyRelations,
					relatedStore,
					relatedField.FieldName,
					storeReverse,
				)
			}
			if err != nil {
				return err
			}
		}

		pRecord := record.ProxyRecord()
		relations := pRecord.Expand()
		maps.Copy(relations, proxyRelations)
		pRecord.SetExpand(relations)
	}
	return nil
}

func (r *RelationStore) expandSingleRelation(
	record core.RecordProxy,
	proxyRelations map[string]any,
	relatedStore RecordStore,
	relatedFieldName string,
	storeReverse bool,
) error {
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
	if storeReverse {
		r.storeReverseRelation(pRecord, relatedPRecord, relatedFieldName)
	}

	return nil
}

func (r *RelationStore) expandMultiRelation(
	record core.RecordProxy,
	proxyRelations map[string]any,
	relatedStore RecordStore,
	relatedFieldName string,
	storeReverse bool,
) error {
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
		if storeReverse {
			r.storeReverseRelation(pRecord, relatedPRecord, relatedFieldName)
		}
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
