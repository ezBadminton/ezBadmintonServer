package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type CourtStore struct {
	list []*Court
	// court id -> match data id
	occupied map[string]string

	// Before court is assigned
	onCourtAssign *hook.Hook[*CourtEvent]
	// After court assignment. After e.Next() the court assignment has been persisted.
	onAfterCourtAssign *hook.Hook[*CourtEvent]

	// Before court is unassigned
	onCourtUnassign *hook.Hook[*CourtEvent]
	// After court unassignment. After e.Next() the unassignment has been persisted.
	onAfterCourtUnassign *hook.Hook[*CourtEvent]
}

func newCourtStore(scheduler *MatchScheduler) *CourtStore {
	courtProxyStore, _ := store.FindRecordStore[Court]()

	courts := slices.Clone(courtProxyStore.RecordList)
	slices.SortFunc(courts, compareCourts)
	occupied := make(map[string]string)
	for m := range scheduler.schedule.IterateMatches() {
		if m.ScheduleStatus == InProgress {
			occupied[m.Match.Court().Id] = m.Id
		}
	}

	courtStore := &CourtStore{
		list:                 courts,
		occupied:             occupied,
		onCourtAssign:        &hook.Hook[*CourtEvent]{},
		onAfterCourtAssign:   &hook.Hook[*CourtEvent]{},
		onCourtUnassign:      &hook.Hook[*CourtEvent]{},
		onAfterCourtUnassign: &hook.Hook[*CourtEvent]{},
	}

	courtProxyStore.RegisterCreateHander(courtStore.created)
	courtProxyStore.RegisterUpdateHandler(courtStore.updated)

	return courtStore
}

func (s *CourtStore) deleteCourt(app core.App, court *Court) error {
	if s.isOccupied(court) {
		return errors.New("can not delete occupied court")
	}

	if err := app.Delete(court); court != nil {
		return err
	}

	s.list = slices.DeleteFunc(
		s.list,
		func(c *Court) bool { return c.Id == court.Id },
	)

	return nil
}

func (s *CourtStore) deleteGymnasium(app core.App, gym *Gymnasium) error {
	courts := findCourtsOfGymnasium(gym)
	for _, c := range courts {
		if s.isOccupied(c) {
			return errors.New("can not delete gymnasium while courts are occupied")
		}
	}

	err := app.RunInTransaction(func(txApp core.App) error {
		for _, c := range courts {
			if err := txApp.Delete(c); err != nil {
				return err
			}
		}
		if err := txApp.Delete(gym); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	ids := idList(courts)
	finder := func(c *Court) bool {
		return slices.Contains(ids, c.Id)
	}
	s.list = slices.DeleteFunc(s.list, finder)

	return nil
}

func (s *CourtStore) isOccupied(court *Court) bool {
	_, ok := s.occupied[court.Id]
	return ok
}

// Returns the next available court
func (s *CourtStore) nextCourt() *Court {
	for _, c := range s.list {
		if !s.isOccupied(c) {
			return c
		}
	}
	return nil
}

func (s *CourtStore) assignCourtToMatch(app core.App, matchData *MatchData, optCourt *Court) error {
	event := newCourtEvent(app, matchData, optCourt)
	return s.onCourtAssign.Trigger(event, s.courtAssignmentHandler)
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus != CourtWait {
			return errors.New("the match is not in the correct state for court assignment")
		}
	*/

	/*
		Scheduler.setMatchScheduleStatus(matchData, Ready, court)
	*/
}

func (s *CourtStore) courtAssignmentHandler(e *CourtEvent) error {
	var court *Court
	if e.Court == nil {
		court = s.nextCourt()
	} else if s.isOccupied(e.Court) {
		return errors.New("can not assign occupied court")
	} else {
		court = e.Court
	}
	if court == nil {
		return errors.New("no court is available")
	}

	e.MatchData.SetCourt(court)
	e.Court = court

	if err := s.onAfterCourtAssign.Trigger(e, (*CourtEvent).saveMatchData); err != nil {
		return err
	}

	return e.Next()
}

func (s *CourtStore) unassignCourt(app core.App, matchData *MatchData) error {
	event := newCourtEvent(app, matchData, nil)
	return s.onCourtUnassign.Trigger(event, s.courtUnassignmentHandler)
	/*
		matchStatus := Scheduler.scheduleStatus(matchData)
		if matchStatus != Ready {
			return errors.New("the match is not in the correct state for court unassignment")
		}
	*/
	/*
		Scheduler.setMatchScheduleStatus(matchData, CourtWait, court)
	*/
}

func (s *CourtStore) courtUnassignmentHandler(e *CourtEvent) error {
	court := e.MatchData.Court()
	e.MatchData.SetCourt(nil)
	e.Court = court

	if err := s.onAfterCourtUnassign.Trigger(e, (*CourtEvent).saveMatchData); err != nil {
		return err
	}

	return e.Next()
}

func (s *CourtStore) created(court *Court) {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	s.list = append(s.list, court)
	slices.SortFunc(s.list, compareCourts)
}

func (s *CourtStore) updated(_, court *Court) {
	defer tops.mu.Unlock()
	tops.mu.Lock()

	i := slices.IndexFunc(s.list, func(c *Court) bool { return c.Id == court.Id })
	s.list[i] = court
	slices.SortFunc(s.list, compareCourts)
}

func findCourtsOfGymnasium(gym *Gymnasium) []*Court {
	parents := store.ListRelationParents(gym.Record)
	courtIds := make([]*Court, len(parents))
	for i, p := range parents {
		courtIds[i], _ = store.FindProxy[Court](p.Id)
	}
	return courtIds
}

// Returns true for status where matches with this status
// occupy their assigned court
func occupationalState(status ScheduleStatus) bool {
	return status == Ready || status == InProgress
}
