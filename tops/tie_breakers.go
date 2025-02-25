package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	got "github.com/ezBadminton/gotournament/core"
	"github.com/pocketbase/pocketbase/core"
)

func addTieBreaker(app core.App, competition *Competition, teams []*Team) error {
	tournament := Tournaments.tournaments[competition.Id]
	if tournament == nil || !tournament.Started {
		return errors.New("can not add tie breaker to tournament that is not running")
	}

	groupKnockout, ok := tournament.Tournament.(*got.GroupKnockout)
	if !ok {
		return errors.New("only group knockout tournaments can have tie-breakers added")
	}
	if groupKnockout.KnockOut.MatchList().MatchesStarted() {
		return errors.New("can not add a tie breaker after the knock out phase started")
	}

	groupPhase := groupKnockout.GroupPhase

	ties := make([][]*got.Slot, 0)
	for _, group := range groupPhase.Groups {
		numUntied := group.FinalRanking.RequiredUntiedRanks
		groupTies := group.FinalRanking.BlockingTies(numUntied)
		ties = append(ties, groupTies...)
	}
	crossTies := groupPhase.FinalRanking.CrossGroupTies()
	ties = append(ties, crossTies...)

	if err := verifyTieBreaker(ties, teams); err != nil {
		return err
	}

	tieBreaker, err := NewProxy[TieBreaker](app)
	if err != nil {
		return err
	}
	tieBreaker.SetTieBreakerRanking(teams)

	compTieBreakers := competition.TieBreakers()
	compTieBreakers = append(compTieBreakers, tieBreaker)
	competition = Clone(competition)
	competition.SetTieBreakers(compTieBreakers)

	err = app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(tieBreaker); err != nil {
			return err
		}
		return txApp.Save(competition)
	})
	if err != nil {
		return err
	}

	insertTieBreaker(teams, groupPhase)
	Tournaments.update(tournament)

	return nil
}

func updateTieBreaker(app core.App, tieBreaker *TieBreaker, teams []*Team) error {
	competition, err := findCompetitionOfTieBreaker(tieBreaker)
	if err != nil {
		return err
	}

	tournament := Tournaments.tournaments[competition.Id]
	groupKnockout := tournament.Tournament.(*got.GroupKnockout)

	if groupKnockout.KnockOut.MatchList().MatchesStarted() {
		return errors.New("can not update a tie breaker after the knock out phase started")
	}
	tieBreakerTeams := tieBreaker.TieBreakerRanking()
	if len(tieBreakerTeams) != len(teams) || !containsAll(tieBreakerTeams, teams) {
		return errors.New("only update the order of a tie breaker, not the contained teams")
	}

	tieBreaker = Clone(tieBreaker)
	tieBreaker.SetTieBreakerRanking(teams)

	if err := app.Save(tieBreaker); err != nil {
		return err
	}

	insertTieBreaker(teams, groupKnockout.GroupPhase)
	Tournaments.update(tournament)

	return nil
}

func deleteTieBreaker(app core.App, tieBreaker *TieBreaker) error {
	competition, err := findCompetitionOfTieBreaker(tieBreaker)
	if err != nil {
		return err
	}

	tournament := Tournaments.tournaments[competition.Id]
	groupKnockout := tournament.Tournament.(*got.GroupKnockout)
	if groupKnockout.KnockOut.MatchList().MatchesStarted() {
		return errors.New("can not delete a tie breaker after the knock out phase started")
	}

	if err := app.Delete(tieBreaker); err != nil {
		return err
	}

	teams := tieBreaker.TieBreakerRanking()
	revokeTieBreaker(teams, groupKnockout.GroupPhase)
	Tournaments.update(tournament)

	return nil
}

func insertTieBreaker(tieBreaker []*Team, tournament *got.GroupPhase) {
	tieBreakerRanking := teamsToConstantRanking(tieBreaker)
	for _, group := range tournament.Groups {
		group.FinalRanking.AddTieBreaker(tieBreakerRanking)
	}
	tournament.FinalRanking.AddTieBreaker(tieBreakerRanking)
}

func revokeTieBreaker(tieBreaker []*Team, tournament *got.GroupPhase) {
	tieBreakerRanking := teamsToConstantRanking(tieBreaker)
	for _, group := range tournament.Groups {
		group.FinalRanking.RemoveTieBreaker(tieBreakerRanking)
	}
	tournament.FinalRanking.RemoveTieBreaker(tieBreakerRanking)
}

func verifyTieBreaker(ties [][]*got.Slot, tieBreaker []*Team) error {
	var fits bool
	for _, tie := range ties {
		tiedTeams := make([]*Team, 0)
		for _, slot := range tie {
			if slot.Player == nil {
				continue
			}
			team := slot.Player.(TournamentPlayer).Team
			tiedTeams = append(tiedTeams, team)
		}
		if len(tieBreaker) == len(tiedTeams) && containsAll(tieBreaker, tiedTeams) {
			fits = true
			break
		}
	}
	if !fits {
		return errors.New("the tie breaker is not applicable to any of the present ties")
	}

	return nil
}

func findCompetitionOfTieBreaker(tieBreaker *TieBreaker) (*Competition, error) {
	parents := store.ListRelationParents(tieBreaker.Record)
	for _, p := range parents {
		comp, err := store.FindProxy[Competition](p.Id)
		if err == nil {
			return comp, nil
		}
	}
	return nil, errors.New("could not find competition of tie breaker")
}
