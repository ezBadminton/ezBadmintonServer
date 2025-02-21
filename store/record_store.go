package store

import (
	"errors"
	"slices"
	"sync"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

var stores map[string]RecordStore
var relationStore *RelationStore

func init() {
	stores = make(map[string]RecordStore)
	relationStore = newRelationStore()
}

func FindRecordStore[P Proxy, PP ProxyP[P]]() (*BaseRecordStore[P, PP], error) {
	collectionName := PP.CollectionName(nil)
	store, err := FindRecordStoreByCollectionName(collectionName)
	if err != nil {
		return nil, err
	}
	baseStore, ok := store.(*BaseRecordStore[P, PP])
	if !ok {
		return nil, errors.New("collection name of the proxy type is not unique")
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

type namedStore struct {
	collectionName string
	store          RecordStore
}

func InitStores(app core.App) error {
	collectionNames := make([]string, 0, 14)
	var err error

	done := make(chan struct{})
	errCh := make(chan error)
	storeCh := make(chan namedStore)

	var wg sync.WaitGroup
	wg.Add(14) // !NB Update this when adding/removing stores

	go initStoreWg[TournamentOrganizer](app, &wg, errCh, storeCh)
	go initStoreWg[AgeGroup](app, &wg, errCh, storeCh)
	go initStoreWg[Club](app, &wg, errCh, storeCh)
	go initStoreWg[Competition](app, &wg, errCh, storeCh)
	go initStoreWg[Court](app, &wg, errCh, storeCh)
	go initStoreWg[Gymnasium](app, &wg, errCh, storeCh)
	go initStoreWg[MatchData](app, &wg, errCh, storeCh)
	go initStoreWg[MatchSet](app, &wg, errCh, storeCh)
	go initStoreWg[Player](app, &wg, errCh, storeCh)
	go initStoreWg[PlayingLevel](app, &wg, errCh, storeCh)
	go initStoreWg[Team](app, &wg, errCh, storeCh)
	go initStoreWg[TieBreaker](app, &wg, errCh, storeCh)
	go initStoreWg[TournamentModeSettings](app, &wg, errCh, storeCh)
	go initStoreWg[TournamentEvent](app, &wg, errCh, storeCh)

	go func() {
		wg.Wait()
		close(done)
	}()

Loop:
	for {
		select {
		case e := <-errCh:
			err = e
		case namedStore := <-storeCh:
			stores[namedStore.collectionName] = namedStore.store
			collectionNames = append(collectionNames, namedStore.collectionName)
		case <-done:
			break Loop
		}
	}

	if err != nil {
		return err
	}

	if err := initRelations(); err != nil {
		return err
	}

	handler := createStoreHook(RecordStore.Created)
	app.OnRecordAfterCreateSuccess(collectionNames...).BindFunc(handler)
	handler = createStoreHook(RecordStore.Updated)
	app.OnRecordAfterUpdateSuccess(collectionNames...).BindFunc(handler)
	handler = createStoreHook(RecordStore.Deleted)
	app.OnRecordAfterDeleteSuccess(collectionNames...).BindFunc(handler)

	errHandler := errorHook(createStoreHook(RecordStore.FailedCreate))
	app.OnRecordAfterCreateError(collectionNames...).BindFunc(errHandler)
	errHandler = errorHook(createStoreHook(RecordStore.FailedUpdate))
	app.OnRecordAfterUpdateError(collectionNames...).BindFunc(errHandler)
	errHandler = errorHook(createStoreHook(RecordStore.FailedDelete))
	app.OnRecordAfterDeleteError(collectionNames...).BindFunc(errHandler)

	realtimeNotifier := createRealtimeNotifierHook(core.ModelEventTypeCreate, RecordStore.RealtimeUpdate)
	app.OnRecordAfterCreateSuccess(collectionNames...).Bind(realtimeNotifier)
	realtimeNotifier = createRealtimeNotifierHook(core.ModelEventTypeUpdate, RecordStore.RealtimeUpdate)
	app.OnRecordAfterUpdateSuccess(collectionNames...).Bind(realtimeNotifier)
	realtimeNotifier = createRealtimeNotifierHook(core.ModelEventTypeDelete, RecordStore.RealtimeUpdate)
	app.OnRecordAfterDeleteSuccess(collectionNames...).Bind(realtimeNotifier)

	app.OnRealtimeMessageSend()

	return nil
}

func initStoreWg[P Proxy, PP ProxyP[P]](app core.App, wg *sync.WaitGroup, errCh chan error, storeCh chan namedStore) {
	defer wg.Done()
	store, err := newStore[P, PP](app)
	if err != nil {
		errCh <- err
		return
	}
	cName := PP.CollectionName(nil)
	storeCh <- namedStore{cName, store}
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

func errorHook(errorHook func(*core.RecordEvent) error) func(*core.RecordErrorEvent) error {
	return func(e *core.RecordErrorEvent) error { return errorHook(&e.RecordEvent) }
}

func createRealtimeNotifierHook(action string, handler func(RecordStore, string, *core.Record) error) *hook.Handler[*core.RecordEvent] {
	return &hook.Handler[*core.RecordEvent]{
		Func: func(e *core.RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			collectionName := e.Record.Collection().Name
			store, err := FindRecordStoreByCollectionName(collectionName)
			if err != nil {
				return err
			}
			if err = handler(store, action, e.Record); err != nil {
				return err
			}

			return nil
		},
		Priority: -99,
	}
}

type RecordStore interface {
	FindRecord(id string) (core.RecordProxy, bool)
	Length() int
	ExpandAll() error
	Created(*core.Record) error
	Updated(*core.Record) error
	Deleted(*core.Record) error
	FailedCreate(*core.Record) error
	FailedUpdate(*core.Record) error
	FailedDelete(*core.Record) error
	RealtimeUpdate(string, *core.Record) error
}

type BaseRecordStore[P Proxy, PP ProxyP[P]] struct {
	RecordMap  map[string]PP
	RecordList []PP
	mu         sync.RWMutex

	createHandlers []func(PP)
	updateHandlers []func(PP, PP)
	deleteHandlers []func(PP)

	failedCreateHandlers,
	failedUpdateHandlers,
	failedDeleteHandlers []func(PP)

	realtimeNotifiers []func(string, PP)
}

func newStore[P Proxy, PP ProxyP[P]](app core.App) (*BaseRecordStore[P, PP], error) {
	records, err := fetchCollection[PP](app)
	if err != nil {
		return nil, err
	}

	recordMap := make(map[string]PP, len(records))
	for _, r := range records {
		recordMap[r.ProxyRecord().Id] = r
	}

	store := &BaseRecordStore[P, PP]{
		RecordMap:  recordMap,
		RecordList: records,
	}

	return store, nil
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
	defer s.mu.RUnlock()
	s.mu.RLock()

	record, ok := s.RecordMap[id]
	return record, ok
}

func (s *BaseRecordStore[_, PP]) FindProxy(id string) (PP, bool) {
	defer s.mu.RUnlock()
	s.mu.RLock()

	record, ok := s.RecordMap[id]
	return record, ok
}

func (s *BaseRecordStore[_, _]) Length() int {
	return len(s.RecordList)
}

func (s *BaseRecordStore[_, PP]) ExpandAll() error {
	if err := ExpandRelations(s.RecordList...); err != nil {
		return err
	}
	return nil
}

func (s *BaseRecordStore[P, PP]) RegisterCreateHander(handler func(created PP)) {
	s.createHandlers = append(s.createHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterUpdateHandler(handler func(old, updated PP)) {
	s.updateHandlers = append(s.updateHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterDeleteHandler(handler func(deleted PP)) {
	s.deleteHandlers = append(s.deleteHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterFailedCreateHander(handler func(created PP)) {
	s.failedCreateHandlers = append(s.failedCreateHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterFailedUpdateHandler(handler func(updated PP)) {
	s.failedUpdateHandlers = append(s.failedUpdateHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterFailedDeleteHandler(handler func(deleted PP)) {
	s.failedDeleteHandlers = append(s.failedCreateHandlers, handler)
}

func (s *BaseRecordStore[P, PP]) RegisterRealtimeNotifier(handler func(action string, record PP)) {
	s.realtimeNotifiers = append(s.realtimeNotifiers, handler)
}

func (s *BaseRecordStore[P, PP]) Created(record *core.Record) error {
	s.mu.Lock()

	proxy, _ := WrapRecord[P, PP](record)
	s.RecordMap[record.Id] = proxy
	s.RecordList = append(s.RecordList, proxy)

	s.mu.Unlock()

	if err := ExpandRelations(proxy); err != nil {
		return err
	}

	for _, handler := range s.createHandlers {
		handler(proxy)
	}

	return nil
}

func (s *BaseRecordStore[P, PP]) Updated(record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the updated record is not part of the store")
	}

	s.mu.Lock()

	old, _ := WrapRecord[P, PP](record.Clone())
	*proxy.ProxyRecord() = *record

	s.mu.Unlock()

	if err := ExpandRelations(proxy); err != nil {
		return err
	}

	for _, handler := range s.updateHandlers {
		handler(old, proxy)
	}

	return nil
}

func (s *BaseRecordStore[_, PP]) Deleted(record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the deleted record is not part of the store")
	}

	s.mu.Lock()

	delete(s.RecordMap, record.Id)
	s.RecordList = slices.DeleteFunc(
		s.RecordList,
		func(p PP) bool { return p.ProxyRecord().Id == record.Id },
	)

	s.mu.Unlock()

	relationStore.RemoveFromRelations(record)

	for _, handler := range s.deleteHandlers {
		handler(proxy)
	}

	return nil
}

func (s *BaseRecordStore[P, PP]) FailedCreate(record *core.Record) error {
	proxy, _ := WrapRecord[P, PP](record)

	for _, handler := range s.failedCreateHandlers {
		handler(proxy)
	}

	return nil
}

func (s *BaseRecordStore[P, PP]) FailedUpdate(record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the not-updated record is not part of the store")
	}

	for _, handler := range s.failedUpdateHandlers {
		handler(proxy)
	}

	return nil
}

func (s *BaseRecordStore[P, PP]) FailedDelete(record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the not-deleted record is not part of the store")
	}

	for _, handler := range s.failedDeleteHandlers {
		handler(proxy)
	}

	return nil
}

func (s *BaseRecordStore[P, PP]) RealtimeUpdate(action string, record *core.Record) error {
	proxy, ok := s.FindProxy(record.Id)
	if !ok {
		return errors.New("the relatime updated record is not part of the store")
	}

	for _, handler := range s.realtimeNotifiers {
		handler(action, proxy)
	}

	return nil
}

// Finds the stored proxy of a record with known proxy type
func FindProxy[P Proxy, PP ProxyP[P]](recordId string) (PP, error) {
	store, err := FindRecordStore[P, PP]()
	if err != nil {
		return nil, err
	}

	found, ok := store.FindProxy(recordId)
	if !ok {
		return nil, errors.New("the record has no stored proxy")
	}

	return found, nil
}

func FindRecord(record *core.Record) core.RecordProxy {
	cName := record.Collection().Name
	store, err := FindRecordStoreByCollectionName(cName)
	if err != nil {
		return nil
	}
	proxy, _ := store.FindRecord(record.Id)
	return proxy
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
