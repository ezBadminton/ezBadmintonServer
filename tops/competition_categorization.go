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
	CollectionName() string
	CategoryGetter() func(*Competition) string
	ComplementCategoryGetter() func(*Competition) string
	AssignToCompetition(*Competition)
	IsInUse(*TournamentEvent) bool
	IsOfCategory(*Competition) bool
	Disable(*TournamentEvent)
}

type CategorizationManager struct {
	// Before one or both of the competition categorizations flip. After e.Next() the competitions have been persisted.
	onCategorizationChange *hook.Hook[*CategorizationEvent]

	// Before a category is deleted. After e.Next() the deletion is persisted and all competitions of that category handled.
	onCategoryDelete *hook.Hook[*CategoryDeleteEvent]
}

func newCategorizationManager(settingsManager *EventSettingsManager) *CategorizationManager {
	m := &CategorizationManager{
		onCategorizationChange: &hook.Hook[*CategorizationEvent]{},
		onCategoryDelete:       &hook.Hook[*CategoryDeleteEvent]{},
	}

	settingsManager.onSettingsChange.BindFunc(m.handleSettingsChange)

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

	app := se.App
	err := se.App.RunInTransaction(func(txApp core.App) error {
		se.App = txApp
		ce := newCategorizationEvent(
			se.App,
			ageGroupsNew,
			playingLevelsNew,
			ageGroupsOld != ageGroupsNew,
			playingLevelsOld != playingLevelsNew,
			competitions,
		)
		err := m.onCategorizationChange.Trigger(ce,
			m.handleCategorizationEnable,
			m.handleCategorizationDisable,
			func(ce *CategorizationEvent) error {
				ce.syncSettingsParent(se)
				defer ce.syncToSettingsParent(se)
				return se.Next()
			},
		)
		ce.syncSettingsParent(se)
		return err
	})
	se.App = app
	return err
}

func (m *CategorizationManager) handleCategorizationEnable(e *CategorizationEvent) error {
	ageGroupsEnabled := e.UseAgeGroups && e.AgeGroupsFlipped
	playingLevelsEnabled := e.UsePlayingLevels && e.PlayingLevelsFlipped
	if !ageGroupsEnabled && !playingLevelsEnabled {
		return e.Next()
	}

	var ageGroup *AgeGroup
	var playingLevel *PlayingLevel

	if ageGroupsEnabled {
		ageGroup = findDefaultCategory(compareAgeGroups)
		if ageGroup == nil {
			return errors.New("can not enable age group categorization with no age groups present")
		}
	}
	if playingLevelsEnabled {
		playingLevel = findDefaultCategory(comparePlayingLevels)
		if playingLevel == nil {
			return errors.New("can not enable playing level categorization with no playing levels present")
		}
	}

	for _, c := range e.Competitions {
		c.SetAgeGroup(ageGroup)
		c.SetPlayingLevel(playingLevel)
		if err := e.App.Save(c); err != nil {
			return err
		}
	}

	return e.Next()
}

func (m *CategorizationManager) handleCategorizationDisable(e *CategorizationEvent) error {
	ageGroupsDisabled := !e.UseAgeGroups && e.AgeGroupsFlipped
	playingLevelsDisabled := !e.UsePlayingLevels && e.PlayingLevelsFlipped
	if !ageGroupsDisabled && !playingLevelsDisabled {
		return e.Next()
	}

	var remainingCategorization func(*Competition) string
	if e.UseAgeGroups {
		remainingCategorization = (*AgeGroup).CategoryGetter(nil)
	}
	if e.UsePlayingLevels {
		remainingCategorization = (*PlayingLevel).CategoryGetter(nil)
	}

	mergeGroups := groupCompetitions(e.Competitions, remainingCategorization)
	if ageGroupsDisabled {
		for _, c := range e.Competitions {
			c.SetAgeGroup(nil)
		}
	}
	if playingLevelsDisabled {
		for _, c := range e.Competitions {
			c.SetPlayingLevel(nil)
		}
	}

	for _, group := range mergeGroups {
		if err := e.Next(); err != nil {
			return err
		}
		mergeTarget := mergeTarget(group)
		if err := mergeCompetitionGroup(mergeTarget, group, e.App); err != nil {
			return err
		}
	}
	return e.Next()
}

func (m *CategorizationManager) handleCategoryDelete(re *core.RecordRequestEvent) error {
	deleted := store.FindRecord(re.Record).(Category)
	replacement, _ := re.RequestEvent.Get("replacement").(Category)

	app := re.App
	err := re.App.RunInTransaction(func(txApp core.App) error {
		re.App = txApp
		de := newCategoryDeleteEvent(re.App, deleted, replacement)
		err := m.onCategoryDelete.Trigger(de,
			m.handleCategoryReplacement,
			func(de *CategoryDeleteEvent) error {
				de.syncParent(re)
				defer de.syncToParent(re)
				return re.Next()
			},
		)
		de.syncParent(re)
		return err
	})
	re.App = app
	return err
}

func (m *CategorizationManager) handleCategoryReplacement(e *CategoryDeleteEvent) error {
	competitionStore, _ := store.FindRecordStore[Competition]()
	if competitionStore.Length() == 0 {
		return e.Next()
	}
	competitions := make([]*Competition, competitionStore.Length())
	for i, c := range competitionStore.ListRecords() {
		competitions[i] = Clone(c)
	}

	eventStore, _ := store.FindRecordStore[TournamentEvent]()
	settings := eventStore.ListRecords()[0]

	if !e.Category.IsInUse(settings) {
		return e.Next()
	}

	cName := e.Category.CollectionName()
	categoryStore, _ := store.FindRecordStoreByCollectionName(cName)

	if categoryStore.Length() == 1 {
		oldSettings := Clone(settings)
		e.Category.Disable(settings)

		ce := newCategorizationEvent(
			e.App,
			settings.UseAgeGroups(),
			settings.UsePlayingLevels(),
			oldSettings.UseAgeGroups() != settings.UseAgeGroups(),
			oldSettings.UsePlayingLevels() != settings.UsePlayingLevels(),
			competitions,
		)

		err := m.onCategorizationChange.Trigger(ce,
			m.handleCategorizationDisable,
			func(ce *CategorizationEvent) error {
				if err := ce.App.Save(settings); err != nil {
					return err
				}
				ce.syncDeleteParent(e)
				defer ce.syncToDeleteParent(e)
				return e.Next()
			},
		)
		ce.syncDeleteParent(e)
		return err
	}

	competitionsOfDeleted := competitionsOfCategory(competitions, e.Category)

	if e.Replacement == nil {
		for _, c := range competitionsOfDeleted {
			if err := e.App.Delete(c); err != nil {
				return err
			}
		}
		return e.Next()
	}

	competitionsOfReplacement := competitionsOfCategory(competitions, e.Replacement)

	competitionsToMerge := slices.Concat(competitionsOfReplacement, competitionsOfDeleted)

	otherCategorization := e.Category.ComplementCategoryGetter()

	mergeGroups := groupCompetitions(competitionsToMerge, otherCategorization)
	for _, group := range mergeGroups {
		if err := mergeCategoryReplacement(e.App, group, e.Replacement); err != nil {
			return err
		}
	}
	return e.Next()
}

func mergeCategoryReplacement(txApp core.App, mergeGroup []*Competition, replacement Category) error {
	mergeTarget := mergeGroup[0]

	if len(mergeGroup) == 1 {
		replacement.AssignToCompetition(mergeTarget)
		return txApp.Save(mergeTarget)
	} else {
		return mergeCompetitionGroup(mergeTarget, mergeGroup, txApp)
	}
}

func competitionsOfCategory(competitions []*Competition, category Category) []*Competition {
	categoryCompetitions := make([]*Competition, 0)
	for _, c := range competitions {
		if category.IsOfCategory(c) {
			categoryCompetitions = append(categoryCompetitions, c)
		}
	}
	return categoryCompetitions
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

	teamPlayers := make(map[*Team][]*Player)
	for _, t := range allTeams {
		teamPlayers[t] = t.Players()
	}

	adoptedTeams := make([]*Team, 0)
	newTeams := make([]*Team, 0)
	droppedTeams := make([]*Team, 0)

	unadoptedTeamSet := make(map[*Team]any, len(allTeams))
	unadoptedPlayerSet := make(map[*Player]any, len(allTeams))
	adoptedPlayerSet := make(map[*Player]any, len(allTeams))

	for _, team := range allTeams {
		unadoptedTeamSet[team] = struct{}{}
		for _, player := range teamPlayers[team] {
			unadoptedPlayerSet[player] = struct{}{}
		}
	}

	// First pass: Adopt all teams that don't cause a player to
	// be registered twice
Loop:
	for _, t := range allTeams {
		for _, p := range teamPlayers[t] {
			if _, ok := adoptedPlayerSet[p]; ok {
				continue Loop
			}
		}

		adoptedTeams = append(adoptedTeams, t)
		delete(unadoptedTeamSet, t)
		for _, p := range teamPlayers[t] {
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
	var mergeTarget *Competition

	if t := standOut(competitions, (*Competition).Registrations, notEmpty); t != nil {
		mergeTarget = t
	} else if t := standOut(competitions, (*Competition).Draw, notEmpty); t != nil {
		mergeTarget = t
	} else if t := standOut(competitions, (*Competition).Seeds, notEmpty); t != nil {
		mergeTarget = t
	} else if t := standOut(competitions, (*Competition).TournamentModeSettings, notNil); t != nil {
		mergeTarget = t
	} else {
		mergeTarget = competitions[0]
	}

	i := slices.Index(competitions, mergeTarget)
	if i != 0 {
		competitions[0], competitions[i] = competitions[i], competitions[0]
	}

	return mergeTarget
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

func findDefaultCategory[P Proxy, PP ProxyP[P]](comparator func(a, b PP) int) PP {
	store, _ := store.FindRecordStore[P, PP]()
	categories := store.ListRecords()
	if len(categories) == 0 {
		return nil
	}
	c := slices.MinFunc(categories, comparator)
	return c
}
