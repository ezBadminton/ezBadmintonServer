package collection

import (
	"errors"
	"slices"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	caches = make(map[string]RecordCache)
	reverseRelations = make(map[string]map[reverseRelation]any)
}

// collection name -> cache
var caches map[string]RecordCache

type reverseRelation struct {
	// Cache that the recordId is in
	cache RecordCache
	// ID of the record that holds the relation
	recordId string
	// name of the field where the relation if found
	relationFieldName string
}

// record ID -> relations that it is in
var reverseRelations map[string]map[reverseRelation]any

type RecordCache interface {
	FindRecord(string) (core.RecordProxy, bool)
	Created(*core.Record) error
	Updated(*core.Record) error
	Deleted(*core.Record) error
}

type RecordList[P core.RecordProxy] []P
type RecordMap[P core.RecordProxy] map[string]P

type Cache[P core.RecordProxy] struct {
	RecordList[P]
	RecordMap[P]

	cName string
	app   core.App
}

func InitCache[P core.RecordProxy](app core.App) error {
	c := &Cache[P]{app: app}
	c.RecordList = make(RecordList[P], 0)
	c.cName = FromProxy(c.RecordList)

	_, ok := caches[c.cName]
	if ok {
		return errors.New("cache for this collection already exists")
	}
	caches[c.cName] = c

	if err := c.FetchAll(); err != nil {
		return err
	}

	return nil
}

func FindCache(collectionName string) (RecordCache, error) {
	cache, ok := caches[collectionName]
	if !ok {
		return nil, errors.New("cache could not be found")
	}
	return cache, nil
}

func (c *Cache[P]) FindRecord(id string) (core.RecordProxy, bool) {
	record, ok := c.RecordMap[id]
	return record, ok
}

func (c *Cache[P]) FetchAll() error {
	collectionFetch := c.app.RecordQuery(c.cName)
	if err := collectionFetch.All(&c.RecordList); err != nil {
		return err
	}
	c.RecordMap = make(RecordMap[P], len(c.RecordList))
	for _, r := range c.RecordList {
		c.RecordMap[r.ProxyRecord().Id] = r
	}
	return nil
}

func (c *Cache[P]) Created(record *core.Record) error {
	var proxy P
	proxy.SetProxyRecord(record)
	c.RecordMap[record.Id] = proxy
	c.RecordList = append(c.RecordList, proxy)
	if err := c.ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (c *Cache[P]) Updated(record *core.Record) error {
	proxy, ok := c.RecordMap[record.Id]
	if !ok {
		return errors.New("the updated record is not part of the cache")
	}

	pRecord := proxy.ProxyRecord()
	*pRecord = *record
	if err := c.ExpandRelations(proxy); err != nil {
		return err
	}

	return nil
}

func (c *Cache[P]) Deleted(record *core.Record) error {
	_, ok := c.RecordMap[record.Id]
	if !ok {
		return errors.New("the deleted record is not part of the cache")
	}

	delete(c.RecordMap, record.Id)
	c.RecordList = slices.DeleteFunc(c.RecordList, func(p P) bool { return p.ProxyRecord().Id == record.Id })
	if err := removeReverseRelations(record); err != nil {
		return err
	}

	return nil
}

func (c *Cache[P]) ExpandRelations(records ...P) error {
	relations := Relations[c.cName]

	for _, record := range records {
		resetReverseRelations(record.ProxyRecord())
	}

	for fieldName, relation := range relations {
		relatedCache, ok := caches[relation.cName]
		if !ok {
			return errors.New("the related collection is not cached")
		}

		for _, record := range records {
			if relation.isMulti {
				if err := expandMultiRelation(record, fieldName, relatedCache); err != nil {
					return err
				}
			} else {
				if err := expandSingleRelation(record, fieldName, relatedCache); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func expandSingleRelation(record core.RecordProxy, relationFieldName string, relatedCache RecordCache) error {
	pRecord := record.ProxyRecord()
	relatedId := pRecord.GetString(relationFieldName)
	if relatedId == "" {
		return nil
	}
	relatedRecord, ok := relatedCache.FindRecord(relatedId)
	if !ok {
		return errors.New("the related record is not cached")
	}
	relatedPRecord := relatedRecord.ProxyRecord()

	e := pRecord.Expand()
	e[relationFieldName] = relatedPRecord
	pRecord.SetExpand(e)

	addReverseRelation(pRecord, relatedPRecord, relatedCache, relationFieldName)
	return nil
}

func expandMultiRelation(record core.RecordProxy, relationFieldName string, relatedCache RecordCache) error {
	pRecord := record.ProxyRecord()
	relatedIds := pRecord.GetStringSlice(relationFieldName)
	if len(relatedIds) == 0 {
		return nil
	}
	relatedRecords := make([]*core.Record, 0)
	for _, id := range relatedIds {
		relatedRecord, ok := relatedCache.FindRecord(id)
		if !ok {
			return errors.New("the related record is not cached")
		}
		relatedPRecord := relatedRecord.ProxyRecord()
		relatedRecords = append(relatedRecords, relatedPRecord)
		addReverseRelation(pRecord, relatedPRecord, relatedCache, relationFieldName)
	}

	e := pRecord.Expand()
	e[relationFieldName] = relatedRecords
	pRecord.SetExpand(e)
	return nil
}

// record the relation in reverse so it can be undone when the relatedRecord is deleted
func addReverseRelation(record, relatedRecord *core.Record, relatedCache RecordCache, relationFieldName string) {
	_, ok := reverseRelations[relatedRecord.Id]
	if !ok {
		reverseRelations[relatedRecord.Id] = make(map[reverseRelation]any)
	}
	reverseRelation := reverseRelation{
		cache:             relatedCache,
		recordId:          record.Id,
		relationFieldName: relationFieldName,
	}
	reverseRelations[relatedRecord.Id][reverseRelation] = struct{}{}
}

// Remove the relatedRecord from any relations that it is in
func removeReverseRelations(relatedRecord *core.Record) error {
	relations := reverseRelations[relatedRecord.Id]
	for relation := range relations {
		record, ok := relation.cache.FindRecord(relation.recordId)
		if !ok {
			return errors.New("the relation containing record is not cached")
		}
		pRecord := record.ProxyRecord()
		e := pRecord.Expand()
		switch relFieldValue := e[relation.relationFieldName].(type) {
		case *core.Record:
			delete(e, relation.relationFieldName)
		case []*core.Record:
			relFieldValue = slices.DeleteFunc(relFieldValue, func(elem *core.Record) bool { return elem.Id == relatedRecord.Id })
			e[relation.relationFieldName] = relFieldValue
		}
		pRecord.SetExpand(e)
	}
	delete(reverseRelations, relatedRecord.Id)
	return nil
}

// Remove the reverse relations that point to the record
func resetReverseRelations(record *core.Record) {
	for _, reverseRelationMap := range reverseRelations {
		for reverseRelation := range reverseRelationMap {
			if reverseRelation.recordId == record.Id {
				delete(reverseRelationMap, reverseRelation)
			}
		}
	}
}
