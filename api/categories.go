package api

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
)

func BindCategoryHooks(app core.App) {
	url := "/playinglevels/reorder"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("", reorderPlayingLevel)
		return e.Next()
	})

	ageGroupCName := CName[AgeGroup]()
	playingLevelCName := CName[PlayingLevel]()
	app.OnRecordDeleteRequest(ageGroupCName, playingLevelCName).BindFunc(onCategoryDelete)
	app.OnRecordCreateRequest(playingLevelCName).BindFunc(tops.AddPlayingLevel)
	app.OnRecordDeleteRequest(playingLevelCName).BindFunc(tops.DeletePlayingLevel)
}

func onCategoryDelete(e *core.RecordRequestEvent) error {
	if err := readReplacementFromQuery(e); err != nil {
		return err
	}

	return tops.DeleteCategory(e)
}

func readReplacementFromQuery(e *core.RecordRequestEvent) error {
	replacementId := e.Request.URL.Query().Get("replacement")
	if replacementId == "" {
		return nil
	}
	s, err := store.FindRecordStoreByCollectionName(e.Collection.Name)
	if err != nil {
		return err
	}
	replacement, ok := s.FindRecord(replacementId)
	if !ok {
		return errors.New("the replacement category ID can not be found")
	}
	e.RequestEvent.Set("replacement", replacement)
	return nil
}

func reorderPlayingLevel(e *core.RequestEvent) error {
	data := struct {
		From int `json:"from"`
		To   int `json:"to"`
	}{-1, -1}
	if err := e.BindBody(&data); err != nil {
		return e.InternalServerError("could not parse body", err)
	}
	if data.From == -1 || data.To == -1 {
		return e.BadRequestError("", errors.New("the JSON body does not contain the 'from' and 'to' integer fields"))
	}
	if err := tops.ReorderPlayingLevel(e.App, data.From, data.To); err != nil {
		return e.BadRequestError("could not reorder PlayingLevel", err)
	}
	return e.NoContent(http.StatusOK)
}
