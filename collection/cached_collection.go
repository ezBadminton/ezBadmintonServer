package collection

import (
	"github.com/pocketbase/pocketbase/core"
)

type RecordList[P core.RecordProxy] []P

type Cached[P core.RecordProxy] struct {
	RecordList[P]

	app core.App
}

func NewCached[P core.RecordProxy](app core.App) *Cached[P] {
	cached := &Cached[P]{app: app}
	return cached
}

func (c *Cached[P]) FetchAll() error {
	c.RecordList = make(RecordList[P], 0)
	collectionName := Names[FromProxy(c.RecordList)]
	collectionFetch := c.app.RecordQuery(collectionName)
	if err := collectionFetch.All(&c.RecordList); err != nil {
		return err
	}
	return nil
}
