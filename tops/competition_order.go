package tops

import (
	. "github.com/ezBadminton/ezBadmintonServer/generated"
)

func compareTournaments(a, b *CompetitionTournament) int {
	return compareCompetitions(a.Competition, b.Competition)
}

func compareCompetitions(a, b *Competition) int {
	if a.Id == b.Id {
		return 0
	}

	ageGroupA, ageGroupB := a.AgeGroup(), b.AgeGroup()
	ageGroupComparison := compareAgeGroups(ageGroupA, ageGroupB)
	if ageGroupComparison != 0 {
		return ageGroupComparison
	}

	playingLevelA, playingLevelB := a.PlayingLevel(), b.PlayingLevel()
	playingLevelComparison := comparePlayingLevels(playingLevelA, playingLevelB)
	if playingLevelComparison != 0 {
		return playingLevelComparison
	}

	teamSizeA, teamSizeB := a.TeamSize(), b.TeamSize()
	teamSizeComparison := compareTeamSize(teamSizeA, teamSizeB)
	if teamSizeComparison != 0 {
		return teamSizeComparison
	}

	genderCatA, genderCatB := a.GenderCategory(), b.GenderCategory()
	genderComparison := compareGenderCategory(genderCatA, genderCatB)

	return genderComparison
}

func compareAgeGroups(a, b *AgeGroup) int {
	if a == nil {
		return 0
	}

	typeA, typeB := a.Type(), b.Type()
	if typeA != typeB {
		// Under, then over
		if typeA == Over {
			return 1
		} else {
			return -1
		}
	}

	ageA, ageB := a.Age(), b.Age()
	if ageA != ageB {
		// Younger, then older
		if ageA > ageB {
			return 1
		} else {
			return -1
		}
	}

	return 0
}

func comparePlayingLevels(a, b *PlayingLevel) int {
	if a == nil {
		return 0
	}

	indexA, indexB := a.Index(), b.Index()
	if indexA == indexB {
		return 0
	}
	// Weaker, then stronger
	if indexA > indexB {
		return -1
	} else {
		return 1
	}
}

func compareTeamSize(a, b int) int {
	if a == b {
		return 0
	}
	// Doubles, then singles
	if a > b {
		return -1
	} else {
		return 1
	}
}

func compareGenderCategory(a, b GenderCategory) int {
	if a == b {
		return 0
	}
	// Women's, then men's, then mixed
	if a > b {
		return 1
	} else {
		return -1
	}
}
