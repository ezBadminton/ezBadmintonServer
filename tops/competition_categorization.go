package tops

import (
	"errors"
	"slices"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

type Category interface {
	AgeGroup | PlayingLevel
}

type CategorizationManager struct {
	// Before one or both of the competition categorizations flip. After e.Next() the competitions have been persisted.
	onCategorizationChange *hook.Hook[*CategorizationEvent]
}

func newCategorizationManager(settingsManager *EventSettingsManager) *CategorizationManager {
	m := &CategorizationManager{
		onCategorizationChange: &hook.Hook[*CategorizationEvent]{},
	}

	settingsManager.onSettingsChange.BindFunc(m.handleSettingsChange)

	m.onCategorizationChange.BindFunc(m.handleCategorizationEnable)
	m.onCategorizationChange.BindFunc(m.handleCategorizationDisable)

	return m
}

func (m *CategorizationManager) handleSettingsChange(se *SettingsEvent) error {
	ageGroupsOld := se.OldSettings.UseAgeGroups()
	ageGroupsNew := se.NewSettings.UseAgeGroups()
	playingLevelsOld := se.OldSettings.UsePlayingLevels()
	playingLevelsNew := se.NewSettings.UsePlayingLevels()

	if ageGroupsOld == ageGroupsNew && playingLevelsOld == playingLevelsNew {
		return se.Next()
	}

	competitionStore, _ := store.FindRecordStore[Competition]()
	competitions := make([]*Competition, competitionStore.Length())
	for i, c := range competitionStore.ListRecords() {
		competitions[i] = Clone(c)
	}

	ce := newCategorizationEvent(
		se,
		ageGroupsNew,
		playingLevelsNew,
		ageGroupsOld != ageGroupsNew,
		playingLevelsOld != playingLevelsNew,
		competitions,
	)
	err := m.onCategorizationChange.Trigger(ce, func(ce *CategorizationEvent) error {
		ce.syncParent(se)
		defer ce.syncToParent(se)
		return se.Next()
	})
	ce.syncParent(se)
	return err
}

func (m *CategorizationManager) handleCategorizationEnable(e *CategorizationEvent) error {
	if !e.UseAgeGroups && !e.UsePlayingLevels {
		return e.Next()
	}

	var ageGroup *AgeGroup
	var playingLevel *PlayingLevel

	if e.UseAgeGroups && e.AgeGroupsFlipped {
		ageGroup = findDefaultCategory(compareAgeGroups)
		if ageGroup == nil {
			return errors.New("can not enable age group categorization with no age groups present")
		}
	}
	if e.UsePlayingLevels && e.PlayingLevelsFlipped {
		playingLevel = findDefaultCategory(comparePlayingLevels)
		if playingLevel == nil {
			return errors.New("can not enable playing level categorization with no playing levels present")
		}
	}

	for _, c := range e.Competitions {
		c.SetAgeGroup(ageGroup)
		c.SetPlayingLevel(playingLevel)
	}

	return e.Next()
}

func (m *CategorizationManager) handleCategorizationDisable(e *CategorizationEvent) error {
	if e.UseAgeGroups && e.UsePlayingLevels {
		return e.Next()
	}

	var remainingCategorization func(*Competition) string
	if e.UseAgeGroups {
		remainingCategorization = ageGroupIdGetter
	}
	if e.UsePlayingLevels {
		remainingCategorization = playingLevelIdGetter
	}

	mergeGroups := groupCompetitions(e.Competitions, remainingCategorization)
	if !e.UseAgeGroups && e.AgeGroupsFlipped {
		for _, c := range e.Competitions {
			c.SetAgeGroup(nil)
		}
	}
	if !e.UsePlayingLevels && e.PlayingLevelsFlipped {
		for _, c := range e.Competitions {
			c.SetPlayingLevel(nil)
		}
	}

	// TODO wrap this transaction around the entire CategorizationEvent
	app := e.App
	err := e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		for _, group := range mergeGroups {
			if err := e.Next(); err != nil {
				return err
			}
			mergeTarget := mergeTarget(group)
			if err := mergeCompetitionGroup(mergeTarget, group, e.App); err != nil {
				return err
			}
		}
		return nil
	})
	e.App = app
	return err
}

type mergeGroup struct {
	GenderCategory  GenderCategory
	CompetitionType string

	// The category of a competition is either its age group or playing level.
	// The string is the ID of the category or empty if not categorized.
	Category string
}

// Groups the given competitions into those of the same discipline
// (discipline is the combination of gender category and competition type)
// and optionally of the the category in the given categorization.
// Leave the categorization func nil to not take category into account.
func groupCompetitions(competitions []*Competition, categorization func(*Competition) string) [][]*Competition {
	groupMap := make(map[mergeGroup][]*Competition)

	for _, c := range competitions {
		group := groupOfCompetition(c, categorization)
		_, ok := groupMap[group]
		if !ok {
			groupMap[group] = make([]*Competition, 0)
		}
		groupMap[group] = append(groupMap[group], c)
	}

	groups := make([][]*Competition, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, group)
	}
	return groups
}

func groupOfCompetition(competition *Competition, categorization func(*Competition) string) mergeGroup {
	genderCategory := competition.GenderCategory()
	teamSize := competition.TeamSize()

	var competitionType string
	if teamSize == 1 {
		competitionType = "singles"
	} else if genderCategory == Mixed {
		competitionType = "mixed"
	} else {
		competitionType = "doubles"
	}

	var category string
	if categorization != nil {
		category = categorization(competition)
	}

	return mergeGroup{
		GenderCategory:  genderCategory,
		CompetitionType: competitionType,
		Category:        category,
	}
}

func mergeCompetitionGroup(target *Competition, group []*Competition, txApp core.App) error {
	adoptedTeams, newTeams, droppedTeams := mergeRegistrations(group)

	for _, c := range group {
		if c == target {
			continue
		}
		if err := txApp.Delete(c); err != nil {
			return err
		}
	}

	for _, t := range newTeams {
		if err := txApp.Save(t); err != nil {
			return err
		}
	}
	for _, t := range droppedTeams {
		if err := txApp.Delete(t); err != nil {
			return err
		}
	}

	mergedRegistrations := slices.Concat(adoptedTeams, newTeams)
	target.SetRegistrations(mergedRegistrations)
	return txApp.Save(target)
}

// Merge the registered teams of the given competitions by returning three lists of teams:
// 1. List of teams that can be directly adopted into the merged competition
// 2. List of teams that need to be newly created
// 3. List of teams that need to be deleted
func mergeRegistrations(competitions []*Competition) ([]*Team, []*Team, []*Team) {
	allTeams := make([]*Team, 0)

	for _, c := range competitions {
		allTeams = append(allTeams, c.Registrations()...)
	}

	if len(allTeams) == 0 {
		return nil, nil, nil
	}

	adoptedTeams := make([]*Team, 0)
	newTeams := make([]*Team, 0)
	droppedTeams := make([]*Team, 0)

	unadoptedTeamSet := make(map[*Team]any, len(allTeams))
	unadoptedPlayerSet := make(map[*Player]any, len(allTeams))
	adoptedPlayerSet := make(map[*Player]any, len(allTeams))

	for _, team := range allTeams {
		unadoptedTeamSet[team] = struct{}{}
		for _, player := range team.Players() {
			unadoptedPlayerSet[player] = struct{}{}
		}
	}

	// First pass: Adopt all teams that don't cause a player to
	// be registered twice
Loop:
	for _, t := range allTeams {
		players := t.Players()
		for _, p := range players {
			if _, ok := adoptedPlayerSet[p]; ok {
				continue Loop
			}
		}

		adoptedTeams = append(adoptedTeams, t)
		delete(unadoptedTeamSet, t)
		for _, p := range players {
			adoptedPlayerSet[p] = struct{}{}
			delete(unadoptedPlayerSet, p)
		}
	}

	// Second pass: Create a new team for each player that was
	// not adopted in the first pass
	teamCollection := allTeams[0].Collection()
	for p := range unadoptedPlayerSet {
		t, _ := WrapRecord[Team](core.NewRecord(teamCollection))
		t.SetPlayers([]*Player{p})
		newTeams = append(newTeams, t)
	}

	// Third pass: Put the unadopted teams in the dropped group
	for t := range unadoptedTeamSet {
		droppedTeams = append(droppedTeams, t)
	}

	return adoptedTeams, newTeams, droppedTeams
}

func mergeTarget(competitions []*Competition) *Competition {
	if len(competitions) == 1 {
		return competitions[0]
	}

	if mergeTarget := standOut(competitions, (*Competition).Registrations, notEmpty); mergeTarget != nil {
		return mergeTarget
	}
	if mergeTarget := standOut(competitions, (*Competition).Draw, notEmpty); mergeTarget != nil {
		return mergeTarget
	}
	if mergeTarget := standOut(competitions, (*Competition).Seeds, notEmpty); mergeTarget != nil {
		return mergeTarget
	}
	if mergeTarget := standOut(competitions, (*Competition).TournamentModeSettings, notNil); mergeTarget != nil {
		return mergeTarget
	}

	return competitions[0]
}

// If exactly one competiton exists where the value from
// the given getter satisfies the standsOut function,
// then that competiton is returned
func standOut[V any](
	competitions []*Competition,
	getter func(*Competition) V,
	standsOut func(V) bool,
) *Competition {
	var standOut *Competition
	for _, c := range competitions {
		if standsOut(getter(c)) {
			if standOut != nil {
				return nil
			} else {
				standOut = c
			}
		}
	}
	return standOut
}

func notEmpty[E any, S ~[]E](s S) bool {
	return len(s) > 0
}

func notNil[E any, P *E](p P) bool {
	return p != nil
}

func ageGroupIdGetter(competition *Competition) string {
	return competition.AgeGroup().Id
}

func playingLevelIdGetter(competition *Competition) string {
	return competition.PlayingLevel().Id
}

func findDefaultCategory[C Category, PP ProxyP[C]](comparator func(a, b PP) int) PP {
	store, _ := store.FindRecordStore[C, PP]()
	categories := store.ListRecords()
	if len(categories) == 0 {
		return nil
	}
	c := slices.MinFunc(categories, comparator)
	return c
}
