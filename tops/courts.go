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

	// Before court assignment. After e.Next() the court assignment has been persisted.
	onCourtAssign *hook.Hook[*CourtEvent]
	// Before court unassignment. After e.Next() the unassignment has been persisted.
	onCourtUnassign *hook.Hook[*CourtEvent]
}

func newCourtStore() *CourtStore {
	return &CourtStore{
		onCourtAssign:   &hook.Hook[*CourtEvent]{},
		onCourtUnassign: &hook.Hook[*CourtEvent]{},
	}
}

func (s *CourtStore) init(
	scheduler *MatchScheduler,
	matchManager *MatchManager,
	tournamentStore *TournamentStore,
) {
	courtProxyStore, _ := store.FindRecordStore[Court]()

	s.list = slices.Clone(courtProxyStore.RecordList)
	slices.SortFunc(s.list, compareCourts)
	s.occupied = make(map[string]string)
	for m := range scheduler.schedule.IterateMatches() {
		if m.ScheduleStatus == InProgress {
			s.occupied[m.Match.Court().Id] = m.Id
		}
	}

	courtProxyStore.RegisterCreateHander(s.created)
	courtProxyStore.RegisterUpdateHandler(s.updated)

	matchManager.onScoreSet.BindFunc(s.handleScoreSet)
	matchManager.onEnd.BindFunc(s.handleMatchEnd)
	// Priority after schedule status update
	matchManager.onReset.Bind(priorityHandler(s.handleMatchReset, -2))

	tournamentStore.onStop.BindFunc(s.handleTournamentStop)
}

func (s *CourtStore) deleteCourt(e *core.RecordRequestEvent) error {
	court, err := store.FindProxy[Court](e.Record.Id)
	if err != nil {
		return err
	}
	if s.isOccupied(court) {
		return errors.New("can not delete occupied court")
	}

	if err := e.Next(); err != nil {
		return err
	}

	s.list = slices.DeleteFunc(
		s.list,
		func(c *Court) bool { return c.Id == court.Id },
	)

	return nil
}

func (s *CourtStore) deleteGymnasium(e *core.RecordRequestEvent) error {
	gym, err := store.FindProxy[Gymnasium](e.Record.Id)
	if err != nil {
		return err
	}
	courts := findCourtsOfGymnasium(gym)
	for _, c := range courts {
		if s.isOccupied(c) {
			return errors.New("can not delete gymnasium while courts are occupied")
		}
	}

	app := e.App
	err = e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		for _, c := range courts {
			if err := txApp.Delete(c); err != nil {
				return err
			}
		}
		return e.Next()
	})
	e.App = app
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
	err := s.onCourtAssign.Trigger(event,
		s.courtAssignmentHandler,
		(*CourtEvent).saveMatchData,
		s.storeAssignment,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
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

	return e.Next()
}

func (s *CourtStore) storeAssignment(e *CourtEvent) error {
	s.occupied[e.Court.Id] = e.MatchData.Id
	return e.Next()
}

func (s *CourtStore) unassignCourt(app core.App, matchData *MatchData) error {
	event := newCourtEvent(app, matchData, nil)
	err := s.onCourtUnassign.Trigger(event,
		s.courtUnassignmentHandler,
		(*CourtEvent).saveMatchData,
		s.storeUnassignment,
	)
	if err == nil {
		event.TriggerRealtimeNotifications()
	}
	return err
}

func (s *CourtStore) courtUnassignmentHandler(e *CourtEvent) error {
	court := e.MatchData.Court()
	e.MatchData.SetCourt(nil)
	e.Court = court

	return e.Next()
}

func (s *CourtStore) storeUnassignment(e *CourtEvent) error {
	delete(s.occupied, e.Court.Id)
	return e.Next()
}

func (s *CourtStore) handleMatchEnd(e *MatchEvent) error {
	if err := e.Next(); err != nil {
		return err
	}
	delete(s.occupied, e.MatchData.Court().Id)
	return nil
}

func (s *CourtStore) handleScoreSet(e *ScoreEvent) error {
	matchEnding := e.MatchData.EndTime().IsZero()
	if err := e.Next(); err != nil {
		return err
	}
	if matchEnding {
		delete(s.occupied, e.MatchData.Court().Id)
	}
	return nil
}

// Allow the match to retake the court it had if it
// is not occupied
func (s *CourtStore) handleMatchReset(e *MatchResetEvent) error {
	currentCourt := e.MatchData.Court()
	courtOccupied := s.isOccupied(currentCourt)

	if courtOccupied {
		e.MatchData.SetCourt(nil)
	}

	if err := e.Next(); err != nil {
		return err
	}

	if !courtOccupied {
		// Re-assign the court
		courtEvent := newCourtEvent(e.App, e.MatchData, currentCourt)
		err := s.onCourtAssign.Trigger(courtEvent,
			s.storeAssignment,
		)
		if err == nil {
			courtEvent.TriggerRealtimeNotifications()
		}
	}

	for _, match := range e.DependantMatches {
		event := newCourtEvent(e.App, match.Match, nil)
		event.FromReset = true
		err := s.onCourtUnassign.Trigger(event,
			s.courtUnassignmentHandler,
			(*CourtEvent).saveMatchData,
			s.storeUnassignment,
		)
		if err == nil {
			event.TriggerRealtimeNotifications()
		}
	}

	return nil
}

func (s *CourtStore) handleTournamentStop(e *PlanEvent) error {
	if err := e.Next(); err != nil {
		return err
	}

	for _, m := range e.MatchData {
		court := m.Court()
		if court == nil {
			continue
		}
		if m.Id == s.occupied[court.Id] {
			delete(s.occupied, court.Id)
		}
	}
	return nil
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
