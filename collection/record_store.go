package collection

import (
	"errors"
	"slices"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/store"
)

var stores map[string]RecordStore
var relationStore *RelationStore

func init() {
	stores = make(map[string]RecordStore)
	relationStore = newRelationStore()
}

func FindRecordStore(collectionName string) (RecordStore, error) {
	store, ok := stores[collectionName]
	if !ok {
		return nil, errors.New("record store of this collection does not exist")
	}
	return store, nil
}

type RecordStore interface {
	FindRecord(id string) (core.RecordProxy, bool)
	RecordList() []core.RecordProxy
	Length() int
	CollectionName() string
	Created(*core.Record) error
	Updated(*core.Record) error
	Deleted(*core.Record) error
}

type BaseRecordStore[P core.RecordProxy] struct {
	*store.Store[string, P]
	recordList     []core.RecordProxy
	collectionName string
}

func InitStore[P core.RecordProxy](records []P) error {
	collectionName := CollectionNameFromProxy(records)
	_, ok := stores[collectionName]
	if ok {
		return errors.New("the RecordStore for this collection already exists")
	}

	recordMap := make(map[string]P, len(records))
	recordList := make([]core.RecordProxy, len(records))
	for i, r := range records {
		recordMap[r.ProxyRecord().Id] = r
		recordList[i] = r
	}

	stores[collectionName] = &BaseRecordStore[P]{
		Store:          store.New(recordMap),
		recordList:     recordList,
		collectionName: collectionName,
	}

	return nil
}

// This has to be called after all stores have been initialized
// Expands all relations using the stored records so no duplicates exist
func InitRelations() error {
	for _, store := range stores {
		records := store.RecordList()
		if err := relationStore.ExpandRelations(records...); err != nil {
			return err
		}
	}
	return nil
}

func (s *BaseRecordStore[P]) FindRecord(id string) (core.RecordProxy, bool) {
	record, ok := s.Store.GetOk(id)
	return record, ok
}

func (s *BaseRecordStore[P]) RecordList() []core.RecordProxy {
	return s.recordList
}

func (s *BaseRecordStore[P]) Length() int {
	return s.Store.Length()
}

func (s *BaseRecordStore[P]) CollectionName() string {
	return s.collectionName
}

func (s *BaseRecordStore[P]) Created(record *core.Record) error {
	var proxy P = NewProxy(s.collectionName).(P)
	proxy.SetProxyRecord(record)

	s.Store.Set(record.Id, proxy)
	s.recordList = append(s.recordList, proxy)
	if err := relationStore.ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (s *BaseRecordStore[P]) Updated(record *core.Record) error {
	proxy, ok := s.FindRecord(record.Id)
	if !ok {
		return errors.New("the updated record is not part of the cache")
	}

	*proxy.ProxyRecord() = *record
	if err := relationStore.ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (s *BaseRecordStore[P]) Deleted(record *core.Record) error {
	_, ok := s.FindRecord(record.Id)
	if !ok {
		return errors.New("the deleted record is not part of the cache")
	}

	s.Store.Remove(record.Id)
	s.recordList = slices.DeleteFunc(
		s.recordList,
		func(p core.RecordProxy) bool { return p.ProxyRecord().Id == record.Id },
	)

	relationStore.RemoveFromRelations(record)

	return nil
}
