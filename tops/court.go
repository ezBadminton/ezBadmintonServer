package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
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

	return nil
}

func (m *CourtManager) verifyCourtDeletion(court *Court) error {
	if m.isOccupied(court) {
		return errors.New("can not delete occupied court")
	}
	return nil
}

func (m *CourtManager) verifyGymnasiumDeletion(gym *Gymnasium) error {
	courtsIds := findCourtsIdsOfGymnasium(gym)
	for _, id := range courtsIds {
		_, ok := m.occupied[id]
		if ok {
			return errors.New("can not delete gym while courts are occupied")
		}
	}
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
	defer topsMu.Unlock()
	topsMu.Lock()

	m.list = slices.DeleteFunc(m.list, func(c *Court) bool { return c.Id == court.Id })
}

func findCourtsIdsOfGymnasium(gym *Gymnasium) []string {
	parents := store.ListRelationParents(gym.Record)
	courtIds := make([]string, len(parents))
	for i, p := range parents {
		courtIds[i] = p.Id
	}
	return courtIds
}
