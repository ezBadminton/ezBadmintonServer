package api

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/ezBadminton/ezBadmintonServer/tops"
)

func BindCategoryHooks(app core.App) {
	ageGroupCName := CName[AgeGroup]()
	playingLevelCName := CName[PlayingLevel]()
	app.OnRecordDeleteRequest(ageGroupCName, playingLevelCName).BindFunc(onCategoryDelete)
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
