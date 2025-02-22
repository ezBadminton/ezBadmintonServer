package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
)

type CourtManager struct {
	list     []*Court
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
	for _, m := range Schedule.runningMatches {
		occupied[m.Court().Id] = m.Id
	}

	Courts = &CourtManager{
		list:     courts,
		occupied: occupied,
	}

	courtStore.RegisterCreateHander(Courts.created)
	courtStore.RegisterUpdateHandler(Courts.updated)
	courtStore.RegisterDeleteHandler(Courts.deleted)

	courtStore.RegisterFailedDeleteHandler(Courts.deleteFailed)

	gymStore, err := store.FindRecordStore[Gymnasium]()
	if err != nil {
		return err
	}

	gymStore.RegisterDeleteHandler(Courts.gymDeleted)
	gymStore.RegisterFailedDeleteHandler(Courts.gymDeleteFailed)

	return nil
}

func (m *CourtManager) verifyCourtDeletion(court *Court) error {
	if m.isOccupied(court) {
		return errors.New("can not delete occupied court")
	}
	return nil
}

func (m *CourtManager) verifyGymnasiumDeletion(gym *Gymnasium) error {
	courts := findCourtsOfGymnasium(gym)
	for _, court := range courts {
		_, ok := m.occupied[court.Id]
		if ok {
			return errors.New("can not delete gym while courts are occupied")
		}
	}

	for _, court := range courts {
		court.SetRaw(DeleteVerifiedKey, struct{}{})
	}
	gym.SetRaw(CourtsOfGymKey, courts)

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
	matchStatus := Schedule.scheduleStatus(matchData)
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

	matchData, _ = WrapRecord[MatchData](matchData.Clone())
	matchData.SetCourt(court)
	if err := app.Save(matchData); err != nil {
		return err
	}

	Schedule.setMatchScheduleStatus(matchData, Ready)

	return nil
}

func (m *CourtManager) unassignCourt(app core.App, matchData *MatchData) error {
	matchStatus := Schedule.scheduleStatus(matchData)
	if matchStatus != Ready {
		return errors.New("the match is not in the correct state for court unassignment")
	}

	matchData, _ = WrapRecord[MatchData](matchData.Clone())
	matchData.SetCourt(nil)
	if err := app.Save(matchData); err != nil {
		return err
	}

	Schedule.setMatchScheduleStatus(matchData, CourtWait)

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

func (m *CourtManager) deleted(court *Court) {
	d := court.CustomData()[DeleteVerifiedKey]
	if d != nil {
		// Court is being deleted through gym deletion
		return
	}

	defer topsMu.Unlock()

	m.list = slices.DeleteFunc(m.list, func(c *Court) bool { return c.Id == court.Id })
}

func (m *CourtManager) deleteFailed(court *Court) {
	d := court.CustomData()[DeleteVerifiedKey]
	// When it is a gym deletion the lock is kept by the gym deletion call
	if d != nil {
		topsMu.Unlock()
	}
}

func (m *CourtManager) gymDeleted(gym *Gymnasium) {
	defer topsMu.Unlock()

	courts := gym.CustomData()[CourtsOfGymKey].([]*Court)
	ids := idList(courts)

	finder := func(c *Court) bool {
		return slices.Contains(ids, c.Id)
	}
	m.list = slices.DeleteFunc(m.list, finder)
}

func (m *CourtManager) gymDeleteFailed(gym *Gymnasium) {
	defer topsMu.Unlock()

	courts := gym.CustomData()[CourtsOfGymKey].([]*Court)
	for _, court := range courts {
		court.SetRaw(DeleteVerifiedKey, nil)
	}
}

func findCourtsOfGymnasium(gym *Gymnasium) []*Court {
	parents := store.ListRelationParents(gym.Record)
	courtIds := make([]*Court, len(parents))
	for i, p := range parents {
		courtIds[i], _ = store.FindProxy[Court](p.Id)
	}
	return courtIds
}
