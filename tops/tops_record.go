// The tournament operations package contains
// tournament logic and holds data in memory that is derived
// from the persistent records and is needed by the
// tops functions
package tops

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type TopsRecord interface {
	ToMap() map[string]any
}

// In memory imitation of a PocketBase core.Record.
// It is a thin abstraction to send the in-memory
// tops data in a way that the default CRUD client
// of the PB SDK is able to parse
type BaseTopsRecord struct {
	Id      string         `json:"id"`
	Created types.DateTime `json:"created"`
	Updated types.DateTime `json:"updated"`
}

func (r *BaseTopsRecord) ToMap(with map[string]any) map[string]any {
	with["id"] = r.Id
	with["created"] = r.Created
	with["updated"] = r.Updated
	return with
}

func TopsRecordListResponse[S ~[]T, T TopsRecord](records S, e *core.RequestEvent) error {
	data := make([]map[string]any, len(records))
	for i, r := range records {
		data[i] = r.ToMap()
	}
	result := map[string]any{
		"items":   data,
		"perPage": -1, // Does not support pagination
	}
	return e.JSON(http.StatusOK, result)
}
