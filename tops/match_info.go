package tops

import (
	"errors"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	got "github.com/ezBadminton/gotournament/core"
)

func matchFinished(match *got.Match) bool {
	_, err := match.GetWinner()
	// Ignoring ErrEqualScore, is not allowed in badminton
	return !errors.Is(err, got.ErrNoScore)
}

func matchEnded(match *got.Match) bool {
	return !match.EndTime.IsZero()
}

func matchesFinished(matches []*got.Match) bool {
	for _, match := range matches {
		if !matchFinished(match) {
			return false
		}
	}
	return true
}

func matchReady(match *got.Match) bool {
	return match.Location != nil && match.StartTime.IsZero()
}

func matchRunning(match *got.Match) bool {
	return !match.StartTime.IsZero() && !matchFinished(match)
}

func matchStarted(match *got.Match) bool {
	return !match.StartTime.IsZero()
}

func hasCourt(match *got.Match) bool {
	return match.Location != nil
}

func playersInMatch(match *got.Match) []*Player {
	players := make([]*Player, 0, 4)
	for s := range match.Slots {
		if s.Player == nil {
			continue
		}
		team := s.Player.(TournamentPlayer).Team
		players = append(players, team.Players()...)
	}

	return players
}
