package collection

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	caches = make(map[CEnum]recordFinder)
}

type recordFinder interface {
	findId(string) (core.RecordProxy, bool)
}

var caches map[CEnum]recordFinder

type RecordList[P core.RecordProxy] []P
type RecordMap[P core.RecordProxy] map[string]P

type Cache[P core.RecordProxy] struct {
	RecordList[P]
	RecordMap[P]

	collection     CEnum
	collectionName string
	app            core.App
}

func NewCache[P core.RecordProxy](app core.App) *Cache[P] {
	c := &Cache[P]{app: app}
	c.RecordList = make(RecordList[P], 0)
	c.collection = FromProxy(c.RecordList)
	c.collectionName = Names[c.collection]

	_, ok := caches[c.collection]
	if ok {
		panic("cache for this collection already exists")
	}
	caches[c.collection] = c

	return c
}

func FindCache[P core.RecordProxy]() (*Cache[P], bool) {
	collection := FromProxy(new(P))
	cache, ok := caches[collection]
	if !ok {
		return nil, false
	}
	typedCache, ok := cache.(*Cache[P])
	return typedCache, ok
}

func (c *Cache[P]) findId(id string) (core.RecordProxy, bool) {
	record, ok := c.RecordMap[id]
	return record, ok
}

func (c *Cache[P]) FetchAll() error {
	collectionFetch := c.app.RecordQuery(c.collectionName)
	if err := collectionFetch.All(&c.RecordList); err != nil {
		return err
	}
	c.RecordMap = make(RecordMap[P], len(c.RecordList))
	for _, r := range c.RecordList {
		c.RecordMap[r.ProxyRecord().Id] = r
	}
	return nil
}

func (c *Cache[P]) ExpandRelations() error {
	relations := Relations[c.collection]
	for fieldName, relation := range relations {
		relatedCache, ok := caches[relation.collection]
		if !ok {
			return errors.New("the related collection is not cached")
		}

		for _, record := range c.RecordList {
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

func expandSingleRelation(record core.RecordProxy, relationFieldName string, finder recordFinder) error {
	pRecord := record.ProxyRecord()
	relatedId := pRecord.GetString(relationFieldName)
	if relatedId == "" {
		return nil
	}
	relatedRecord, ok := finder.findId(relatedId)
	if !ok {
		return errors.New("the related record is not cached")
	}

	e := pRecord.Expand()
	e[relationFieldName] = relatedRecord.ProxyRecord()
	pRecord.SetExpand(e)
	return nil
}

func expandMultiRelation(record core.RecordProxy, relationFieldName string, finder recordFinder) error {
	pRecord := record.ProxyRecord()
	relatedIds := pRecord.GetStringSlice(relationFieldName)
	if len(relatedIds) == 0 {
		return nil
	}
	relatedRecords := make([]*core.Record, 0)
	for _, id := range relatedIds {
		relatedRecord, ok := finder.findId(id)
		if !ok {
			return errors.New("the related record is not cached")
		}
		relatedRecords = append(relatedRecords, relatedRecord.ProxyRecord())
	}

	e := pRecord.Expand()
	e[relationFieldName] = relatedRecords
	pRecord.SetExpand(e)
	return nil
}
