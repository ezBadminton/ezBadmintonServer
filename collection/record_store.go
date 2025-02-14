package collection

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/store"
)

var stores map[string]RecordStore
var relationStore *RelationStore

func init() {
	stores = make(map[string]RecordStore)
	relationStore = newRelationStore()
}

func FindRecordStore[PP ProxyP[P], P Proxy]() (*BaseRecordStore[P, PP], error) {
	collectionName := PP.CollectionName(nil)
	store, err := FindRecordStoreByCollectionName(collectionName)
	if err != nil {
		return nil, err
	}
	baseStore, ok := store.(*BaseRecordStore[P, PP])
	if !ok {
		return nil, errors.New("collection name of the proxy type is duplicate")
	}
	return baseStore, nil
}

func FindRecordStoreByCollectionName(collectionName string) (RecordStore, error) {
	store, ok := stores[collectionName]
	if !ok {
		return nil, errors.New("record store of this collection does not exist")
	}
	return store, nil
}

func InitStores(app core.App) error {
	if err := initStore[TournamentOrganizer](app); err != nil {
		return err
	}
	if err := initStore[AgeGroup](app); err != nil {
		return err
	}
	if err := initStore[Club](app); err != nil {
		return err
	}
	if err := initStore[Competition](app); err != nil {
		return err
	}
	if err := initStore[Court](app); err != nil {
		return err
	}
	if err := initStore[Gymnasium](app); err != nil {
		return err
	}
	if err := initStore[MatchData](app); err != nil {
		return err
	}
	if err := initStore[MatchSet](app); err != nil {
		return err
	}
	if err := initStore[Player](app); err != nil {
		return err
	}
	if err := initStore[PlayingLevel](app); err != nil {
		return err
	}
	if err := initStore[Team](app); err != nil {
		return err
	}
	if err := initStore[TieBreaker](app); err != nil {
		return err
	}
	if err := initStore[TournamentModeSettings](app); err != nil {
		return err
	}
	if err := initStore[Tournament](app); err != nil {
		return err
	}

	if err := initRelations(); err != nil {
		return err
	}

	app.OnRecordAfterCreateSuccess().BindFunc(createStoreHook(RecordStore.Created))
	app.OnRecordAfterUpdateSuccess().BindFunc(createStoreHook(RecordStore.Updated))
	app.OnRecordAfterDeleteSuccess().BindFunc(createStoreHook(RecordStore.Deleted))

	return nil
}

func createStoreHook(handler func(RecordStore, *core.Record) error) func(*core.RecordEvent) error {
	return func(e *core.RecordEvent) error {
		collectionName := e.Record.Collection().Name
		store, err := FindRecordStoreByCollectionName(collectionName)
		if err != nil {
			return err
		}
		if err = handler(store, e.Record); err != nil {
			return err
		}

		return e.Next()
	}
}

type RecordStore interface {
	FindRecord(id string) (core.RecordProxy, bool)
	Length() int
	ExpandAll() error
	Created(*core.Record) error
	Updated(*core.Record) error
	Deleted(*core.Record) error
}

type BaseRecordStore[P Proxy, PP ProxyP[P]] struct {
	*store.Store[string, PP]
	recordList []PP
}

func initStore[P Proxy, PP ProxyP[P]](app core.App) error {
	records, err := fetchCollection[PP](app)
	if err != nil {
		return err
	}

	collectionName := PP.CollectionName(nil)
	_, ok := stores[collectionName]
	if ok {
		return errors.New("the RecordStore for this collection already exists")
	}

	recordMap := make(map[string]PP, len(records))
	for _, r := range records {
		recordMap[r.ProxyRecord().Id] = r
	}

	stores[collectionName] = &BaseRecordStore[P, PP]{
		Store:      store.New(recordMap),
		recordList: records,
	}

	return nil
}

// This has to be called after all stores have been initialized
// Expands all relations using the stored records so no duplicates exist
func initRelations() error {
	for _, store := range stores {
		if err := store.ExpandAll(); err != nil {
			return err
		}
	}
	return nil
}

func (s *BaseRecordStore[_, _]) FindRecord(id string) (core.RecordProxy, bool) {
	record, ok := s.Store.GetOk(id)
	return record, ok
}

func (s *BaseRecordStore[_, PP]) FindProxy(id string) (PP, bool) {
	record, ok := s.Store.GetOk(id)
	return record, ok
}

func (s *BaseRecordStore[_, _]) Length() int {
	return s.Store.Length()
}

func (s *BaseRecordStore[_, PP]) ExpandAll() error {
	if err := ExpandRelations(s.recordList...); err != nil {
		return err
	}
	return nil
}

func (s *BaseRecordStore[P, PP]) Created(record *core.Record) error {
	proxy, _ := WrapRecord[P, PP](record)

	s.Store.Set(record.Id, proxy)
	s.recordList = append(s.recordList, proxy)
	if err := ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (s *BaseRecordStore[_, PP]) Updated(record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the updated record is not part of the cache")
	}

	*proxy.ProxyRecord() = *record
	if err := ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (s *BaseRecordStore[_, PP]) Deleted(record *core.Record) error {
	_, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the deleted record is not part of the cache")
	}

	s.Store.Remove(record.Id)
	s.recordList = slices.DeleteFunc(
		s.recordList,
		func(p PP) bool { return p.ProxyRecord().Id == record.Id },
	)

	relationStore.RemoveFromRelations(record)

	return nil
}

// Finds the stored proxy of a record
func FindProxy[P Proxy, PP ProxyP[P]](record *core.Record) (PP, error) {
	collectionName := record.Collection().Name
	proxyCollectionName := PP.CollectionName(nil)
	if collectionName != proxyCollectionName {
		return nil, errors.New("the generic proxy type is not of the same collection as the given record")
	}
	store, err := FindRecordStore[PP]()
	if err != nil {
		return nil, err
	}

	found, ok := store.FindProxy(record.Id)
	if !ok {
		return nil, errors.New("the record has no stored proxy")
	}

	return found, nil
}

func fetchCollection[PP ProxyP[P], P Proxy](app core.App) ([]PP, error) {
	records := make([]PP, 0)
	collectionName := PP.CollectionName(nil)
	query := app.RecordQuery(collectionName)

	if err := query.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}
