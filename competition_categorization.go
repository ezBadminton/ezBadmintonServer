package main

import (
	"errors"

	names "github.com/ezBadminton/ezBadmintonServer/schema_names"
	"github.com/pocketbase/pocketbase/core"
)

// HandleEnabledCategorization adds a category (age group or playing level) that has been enabled
// to all competitions. Enabling a categorization without a category being present results in an error.
// All competitions are put into the same category. It is undetermined which category that is.
// For example to enable the age group categorization at least one AgeGroup record has to be in the
// age group collection. When no competitions are present, categorizations can always be enabled.
func HandleEnabledCategorization(ageGroupsEnabled bool, playingLevelsEnabled bool, dao core.App) error {
	if !ageGroupsEnabled && !playingLevelsEnabled {
		return nil
	}

	return dao.RunInTransaction(func(txDao core.App) error {
		competitions, fetchErr := FetchCollection(names.Collections.Competitions, txDao)
		if fetchErr != nil {
			return fetchErr
		}

		if len(competitions) == 0 {
			return nil
		}

		if ageGroupsEnabled {
			if err := addCategoryToCompetitions(
				competitions,
				names.Fields.Competitions.AgeGroup,
				names.Collections.AgeGroups,
				txDao,
			); err != nil {
				return err
			}
		}

		if playingLevelsEnabled {
			if err := addCategoryToCompetitions(
				competitions,
				names.Fields.Competitions.PlayingLevel,
				names.Collections.PlayingLevels,
				txDao,
			); err != nil {
				return err
			}
		}

		if err := ProcessAsModels(competitions, txDao.Save); err != nil {
			return err
		}

		return nil
	})

}

// HandleDisabledCategorization removes a category (age group or playing level) that has been disabled
// from all competitions. It also merges competitions that were previously categorized into one.
// For example when the age group categorization becomes disabled and there exist n men's singles
// competitions in n different age groups, then those are merged.
func HandleDisabledCategorization(ageGroupsDisabled bool, playingLevelsDisabled bool, dao core.App) error {
	if !ageGroupsDisabled && !playingLevelsDisabled {
		return nil
	}

	remainingCategorization := ""

	if !ageGroupsDisabled {
		remainingCategorization = names.Fields.Competitions.AgeGroup
	}
	if !playingLevelsDisabled {
		remainingCategorization = names.Fields.Competitions.PlayingLevel
	}

	return dao.RunInTransaction(func(txDao core.App) error {
		competitions, fetchErr := FetchAndExpandCollection(names.Collections.Competitions, txDao)
		if fetchErr != nil {
			return fetchErr
		}

		var mergeGroups [][]*core.Record = GroupCompetitions(competitions, remainingCategorization)

		if ageGroupsDisabled {
			removeCategoryFromCompetitions(competitions, names.Fields.Competitions.AgeGroup)
		}
		if playingLevelsDisabled {
			removeCategoryFromCompetitions(competitions, names.Fields.Competitions.PlayingLevel)
		}

		if err := ProcessAsModels(competitions, txDao.Save); err != nil {
			return err
		}

		for _, group := range mergeGroups {
			var mergeTarget *core.Record = getMergeTarget(group)

			if err := mergeCompetitionGroup(group, mergeTarget, txDao); err != nil {
				return nil
			}
		}

		return nil
	})
}

// HandleDeletedCategory processes the competitions that are in a category that is about to be deleted.
// If the replacementCategoryId is not an empty string the competition's registration lists are merged
// into the competitions of the replacement category.
func HandleDeletedCategory(deletedCategory *core.Record, replacementCategoryId string, dao core.App) error {
	return dao.RunInTransaction(func(txDao core.App) error {
		competitions, fetchErr := FetchAndExpandCollection(names.Collections.Competitions, txDao)
		if fetchErr != nil {
			return fetchErr
		}

		tournamentQuery := txDao.RecordQuery(names.Collections.Tournaments).Limit(1)

		var tournament *core.Record = &core.Record{}
		if err := tournamentQuery.One(tournament); err != nil {
			return err
		}

		categorizationOptionName := getOptionNameOfCategory(deletedCategory)
		useCategorization := tournament.GetBool(categorizationOptionName)

		if !useCategorization || len(competitions) == 0 {
			return nil
		}

		categories, fetchErr := FetchCollection(deletedCategory.Collection().Name, txDao)
		if fetchErr != nil {
			return fetchErr
		}

		if len(categories) == 1 {
			// Deleting the last category disables the categorization
			tournament.Set(categorizationOptionName, false)
			txDao.Save(tournament)
			return nil
		}

		competitionsOfDeleted := GetCompetitionsOfCategory(competitions, deletedCategory)

		if replacementCategoryId == "" {
			if err := ProcessAsRecords(competitionsOfDeleted, func(competition *core.Record) error {
				return DeleteCompetitionAndTeams(competition, txDao)
			}); err != nil {
				return err
			}
			return nil
		}

		replacementCategory, fetchErr := txDao.FindRecordById(deletedCategory.Collection().Name, replacementCategoryId)
		if fetchErr != nil {
			return fetchErr
		}
		if replacementCategory == nil {
			return errors.New("the replacement category does not exist")
		}

		competitionsOfReplacement := GetCompetitionsOfCategory(competitions, replacementCategory)

		competitionsToMerge := make([]*core.Record, 0, len(competitionsOfDeleted)+len(competitionsOfReplacement))
		competitionsToMerge = append(competitionsToMerge, competitionsOfReplacement...)
		competitionsToMerge = append(competitionsToMerge, competitionsOfDeleted...)

		otherCategorization := getInvertedTypeOfCategory(replacementCategory)
		var mergeGroups [][]*core.Record = GroupCompetitions(competitionsToMerge, otherCategorization)

		for _, group := range mergeGroups {
			if err := mergeCategoryReplacement(group, replacementCategory, txDao); err != nil {
				return err
			}
		}

		return nil
	})
}

func mergeCategoryReplacement(mergeGroup []*core.Record, replacementCategory *core.Record, dao core.App) error {
	mergeTarget := mergeGroup[0]

	if len(mergeGroup) == 1 {
		var categoryType string = getTypeOfCategory(replacementCategory)

		mergeTarget.Set(categoryType, replacementCategory.Id)

		if err := dao.Save(mergeTarget); err != nil {
			return err
		}

	} else if len(mergeGroup) == 2 {
		if err := mergeCompetitionGroup(mergeGroup, mergeTarget, dao); err != nil {
			return err
		}
	}

	return nil
}

// Deletes the competitions that have been merged into the merge target
func deleteMergedCompetitions(mergeGroup []*core.Record, mergeTarget *core.Record, txDao core.App) error {
	for _, competition := range mergeGroup {
		if competition == mergeTarget {
			continue
		}

		if err := txDao.Delete(competition); err != nil {
			return err
		}
	}

	return nil
}

// Remove the given category ("ageGroup" or "playingLevel") from the competitions
func removeCategoryFromCompetitions(competitions []*core.Record, category string) {
	for _, competition := range competitions {
		competition.Set(category, nil)
	}
}

func addCategoryToCompetitions(
	competitions []*core.Record,
	category string,
	categoryCollectionName string,
	dao core.App,
) error {
	collectionFetch := dao.RecordQuery(categoryCollectionName).Limit(1)
	firstCategory := core.Record{}
	if err := collectionFetch.One(&firstCategory); err != nil {
		return err
	}

	for _, competition := range competitions {
		competition.Set(category, firstCategory.Id)
	}

	return nil
}

type CompetitionGroup struct {
	GenderCategory  string
	CompetitionType string

	// The category of a competition is either its age group, playing level.
	// The string is the ID of the category or empty if the competition has no categorization.
	Category string
}

// Groups the given competitions into those of the same discipline
// and optionally of the the category in the given categorization ("ageGroup" or "playingLevel").
// Leave the categorization string empty to not take category into account.
func GroupCompetitions(competitions []*core.Record, categorization string) [][]*core.Record {
	disciplineMap := make(map[CompetitionGroup][]*core.Record)

	for _, competition := range competitions {
		category := GroupOfCompetition(competition, categorization)

		_, exists := disciplineMap[category]
		if !exists {
			disciplineMap[category] = make([]*core.Record, 0, 3)
		}

		disciplineMap[category] = append(disciplineMap[category], competition)
	}

	competitionGroups := make([][]*core.Record, 0, len(disciplineMap))
	for _, group := range disciplineMap {
		competitionGroups = append(competitionGroups, group)
	}

	return competitionGroups
}

// Returns the group that the given competition belongs to.
// By giving a categorization of "playingLevel" or "ageGroup" the group will also adhere to that.
func GroupOfCompetition(competition *core.Record, categorization string) CompetitionGroup {
	genderCategory := competition.GetString(names.Fields.Competitions.GenderCategory)
	teamSize := competition.GetInt(names.Fields.Competitions.TeamSize)

	var competitionType string
	if teamSize == 1 {
		competitionType = "singles"
	} else if genderCategory == "mixed" {
		competitionType = "mixed"
	} else {
		competitionType = "doubles"
	}

	var categoryRecord *core.Record = competition.ExpandedOne(categorization)
	var category string
	switch categoryRecord {
	case nil:
		category = "none"
	default:
		category = categoryRecord.Id
	}

	newCompetitionDiscipline := CompetitionGroup{
		GenderCategory:  genderCategory,
		CompetitionType: competitionType,
		Category:        category,
	}

	return newCompetitionDiscipline
}

// Returns the one competition in a group of competitions that the others should
// be merged into.
func getMergeTarget(competitions []*core.Record) *core.Record {
	if len(competitions) == 1 {
		return competitions[0]
	}

	// Test if there is a standout competition that is the single one that has
	// a registrations list, a draw, seeds or tournament mode settings
	if singleOne := GetSingle(competitions, names.Fields.Competitions.Registrations); singleOne != nil {
		return singleOne
	}
	if singleOne := GetSingle(competitions, names.Fields.Competitions.Draw); singleOne != nil {
		return singleOne
	}
	if singleOne := GetSingle(competitions, names.Fields.Competitions.Seeds); singleOne != nil {
		return singleOne
	}
	if singleOne := GetSingle(competitions, names.Fields.Competitions.TournamentModeSettings); singleOne != nil {
		return singleOne
	}

	return competitions[0]
}

// Merge the registered teams of the given competitions by returning three lists of teams:
// 1. List of teams that can be directly adopted into the merged competition
// 2. List of teams that need to newly created
// 3. List of teams that need to be deleted
func mergeRegistrations(
	competitions []*core.Record,
	mergeTarget *core.Record,
) ([]*core.Record, []*core.Record, []*core.Record) {
	var allTeams []*core.Record = mergeTarget.ExpandedAll(names.Fields.Competitions.Registrations)

	adoptedTeams := []*core.Record{}
	newTeams := []*core.Record{}
	deletedTeams := []*core.Record{}

	if len(allTeams) == 0 {
		return adoptedTeams, newTeams, deletedTeams
	}

	for _, competition := range competitions {
		if competition != mergeTarget {
			allTeams = append(allTeams, competition.ExpandedAll(names.Fields.Competitions.Registrations)...)
		}
	}

	if len(allTeams) == 0 {
		return adoptedTeams, newTeams, deletedTeams
	}

	// Initially all teams and players are unadopted
	unadoptedTeamSet := make(map[*core.Record]struct{}, len(allTeams))

	unadoptedPlayerSet := make(map[*core.Record]struct{}, len(allTeams))
	adoptedPlayerSet := make(map[*core.Record]struct{}, len(allTeams))

	for _, team := range allTeams {
		unadoptedTeamSet[team] = struct{}{}
		for _, player := range team.ExpandedAll(names.Fields.Teams.Players) {
			unadoptedPlayerSet[player] = struct{}{}
		}
	}

	// First pass: Adopt all teams from the full list that don't
	// cause a player to be registered twice
	for _, team := range allTeams {
		alreadyAdopted := false
		for _, player := range team.ExpandedAll(names.Fields.Teams.Players) {
			if _, isAdopted := adoptedPlayerSet[player]; isAdopted {
				alreadyAdopted = true
				break
			}
		}

		if alreadyAdopted {
			continue
		}

		adoptedTeams = append(adoptedTeams, team)
		delete(unadoptedTeamSet, team)
		for _, player := range team.ExpandedAll(names.Fields.Teams.Players) {
			adoptedPlayerSet[player] = struct{}{}
			delete(unadoptedPlayerSet, player)
		}
	}

	// Second pass: Create a new team for each player that was not adopted
	// in the first pass.
	var teamCollection *core.Collection = allTeams[0].Collection()
	for player := range unadoptedPlayerSet {
		newTeam := core.NewRecord(teamCollection)
		newTeam.Set("players", []*core.Record{player})
		newTeams = append(newTeams, newTeam)
	}

	// Third pass: Mark the unadopted teams for deletion.
	for team := range unadoptedTeamSet {
		deletedTeams = append(deletedTeams, team)
	}

	return adoptedTeams, newTeams, deletedTeams
}

func mergeCompetitionGroup(mergeGroup []*core.Record, mergeTarget *core.Record, dao core.App) error {
	return dao.RunInTransaction(func(txDao core.App) error {

		var adoptedTeams, newTeams, deletedTeams []*core.Record = mergeRegistrations(mergeGroup, mergeTarget)

		if err := ProcessAsModels(newTeams, txDao.Save); err != nil {
			return err
		}
		if err := ProcessAsModels(deletedTeams, txDao.Delete); err != nil {
			return err
		}

		var updatedRegistrations []string = make([]string, 0, len(adoptedTeams)+len(newTeams))
		for _, team := range adoptedTeams {
			updatedRegistrations = append(updatedRegistrations, team.Id)
		}
		for _, team := range newTeams {
			updatedRegistrations = append(updatedRegistrations, team.Id)
		}
		mergeTarget.Set(names.Fields.Competitions.Registrations, updatedRegistrations)

		if err := txDao.Save(mergeTarget); err != nil {
			return err
		}
		if err := deleteMergedCompetitions(mergeGroup, mergeTarget, txDao); err != nil {
			return err
		}

		return nil
	})
}

// Returns what type of category the given model represents.
// Either "ageGroup" or "playingLevel"
func getTypeOfCategory(category *core.Record) string {
	switch category.Collection().Name {
	case names.Collections.PlayingLevels:
		return names.Fields.Competitions.PlayingLevel
	case names.Collections.AgeGroups:
		return names.Fields.Competitions.AgeGroup
	}

	return ""
}

// Returns the oppositre ("ageGroup" or "playingLevel") of
// what the given category is.
func getInvertedTypeOfCategory(category *core.Record) string {
	switch category.Collection().Name {
	case names.Collections.PlayingLevels:
		return names.Fields.Competitions.AgeGroup
	case names.Collections.AgeGroups:
		return names.Fields.Competitions.PlayingLevel
	}

	return ""
}

// Returns what the name of the option for the category the given model represents.
// Either "useAgeGroups" or "usePlayingLevels"
func getOptionNameOfCategory(category *core.Record) string {
	switch category.Collection().Name {
	case names.Collections.PlayingLevels:
		return names.Fields.Tournaments.UsePlayingLevels
	case names.Collections.AgeGroups:
		return names.Fields.Tournaments.UseAgeGroups
	}

	return ""
}

func GetCompetitionsOfCategory(competitions []*core.Record, category *core.Record) []*core.Record {
	competitionsOfCategory := make([]*core.Record, 0, 2)

	var categoryType string = getTypeOfCategory(category)

	for _, competition := range competitions {
		if competition.GetString(categoryType) == category.Id {
			competitionsOfCategory = append(competitionsOfCategory, competition)
		}
	}

	return competitionsOfCategory
}
