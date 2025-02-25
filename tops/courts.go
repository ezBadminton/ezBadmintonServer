package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

type CourtManager struct {
	list []*Court
	// court id -> match data id
	occupied map[string]string
}

var Courts *CourtManager

func InitCourts() error {
	courtStore, err := store.FindRecordStore[Court]()
	if err != nil {
		return err
	}

	courts := slices.Clone(courtStore.RecordList)
	slices.SortFunc(courts, compareCourts)
	occupied := make(map[string]string)
	for m := range Scheduler.schedule.IterateMatches() {
		if m.ScheduleStatus == InProgress {
			occupied[m.Match.Court().Id] = m.Id
		}
	}

	Courts = &CourtManager{
		list:     courts,
		occupied: occupied,
	}

	courtStore.RegisterCreateHander(Courts.created)
	courtStore.RegisterUpdateHandler(Courts.updated)

	return nil
}

func (m *CourtManager) deleteCourt(app core.App, court *Court) error {
	if m.isOccupied(court) {
		return errors.New("can not delete occupied court")
	}

	if err := app.Delete(court); court != nil {
		return err
	}

	m.list = slices.DeleteFunc(
		m.list,
		func(c *Court) bool { return c.Id == court.Id },
	)

	return nil
}

func (m *CourtManager) deleteGymnasium(app core.App, gym *Gymnasium) error {
	courts := findCourtsOfGymnasium(gym)
	for _, c := range courts {
		if m.isOccupied(c) {
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
	m.list = slices.DeleteFunc(m.list, finder)

	return nil
}

func (m *CourtManager) isOccupied(court *Court) bool {
	_, ok := m.occupied[court.Id]
	return ok
}

// Returns the next available court
func (m *CourtManager) nextCourt() *Court {
	for _, c := range m.list {
		if !m.isOccupied(c) {
			return c
		}
	}
	return nil
}

func (m *CourtManager) assignCourtToMatch(app core.App, matchData *MatchData, optCourt *Court) error {
	matchStatus := Scheduler.scheduleStatus(matchData)
	if matchStatus != CourtWait {
		return errors.New("the match is not in the correct state for court assignment")
	}

	var court *Court
	if optCourt == nil {
		court = m.nextCourt()
	} else if m.isOccupied(optCourt) {
		return errors.New("can not assign occupied court")
	} else {
		court = optCourt
	}
	if court == nil {
		return errors.New("no court is available")
	}

	matchData = Clone(matchData)
	matchData.SetCourt(court)
	if err := app.Save(matchData); err != nil {
		return err
	}

	Scheduler.setMatchScheduleStatus(matchData, Ready, court)

	return nil
}

func (m *CourtManager) unassignCourt(app core.App, matchData *MatchData) error {
	matchStatus := Scheduler.scheduleStatus(matchData)
	if matchStatus != Ready {
		return errors.New("the match is not in the correct state for court unassignment")
	}

	court := matchData.Court()

	matchData = Clone(matchData)
	matchData.SetCourt(nil)
	if err := app.Save(matchData); err != nil {
		return err
	}

	Scheduler.setMatchScheduleStatus(matchData, CourtWait, court)

	return nil
}

func (m *CourtManager) created(court *Court) {
	defer topsMu.Unlock()
	topsMu.Lock()

	m.list = append(m.list, court)
	slices.SortFunc(m.list, compareCourts)
}

func (m *CourtManager) updated(_, court *Court) {
	defer topsMu.Unlock()
	topsMu.Lock()

	i := slices.IndexFunc(m.list, func(c *Court) bool { return c.Id == court.Id })
	m.list[i] = court
	slices.SortFunc(m.list, compareCourts)
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
