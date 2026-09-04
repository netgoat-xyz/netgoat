package challenge

import "time"

const (
	defaultBaseDifficulty = 18
	defaultMaxDifficulty  = 24
	defaultMismatchBump   = 4
	loadWindow            = time.Second
)

func (s *Store) difficulty(binding SessionBinding) int {
	d := s.loadDifficulty()
	if binding.StackClassMismatch {
		d += s.mismatchBump
	}
	if d > s.maxDifficulty {
		d = s.maxDifficulty
	}
	if d < 1 {
		d = 1
	}
	return d
}

// DifficultyFor is the exported load+mismatch function. User-Agent is
// intentionally not an input: python-requests, go-http-client, and bot
// strings must not raise cost by themselves.
func (s *Store) DifficultyFor(binding SessionBinding) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.now())
	return s.difficulty(binding)
}

func (s *Store) loadDifficulty() int {
	d := s.baseDifficulty
	inFlight := len(s.challenges)
	switch {
	case inFlight >= 512:
		d += 6
	case inFlight >= 128:
		d += 4
	case inFlight >= 32:
		d += 2
	}

	rps := s.recentIssueRateLocked()
	switch {
	case rps >= 400:
		d += 6
	case rps >= 100:
		d += 4
	case rps >= 20:
		d += 2
	}
	if d > s.maxDifficulty {
		return s.maxDifficulty
	}
	return d
}

func (s *Store) noteIssueLocked(now time.Time) {
	s.recentIssues = append(s.recentIssues, now)
	cutoff := now.Add(-loadWindow)
	i := 0
	for i < len(s.recentIssues) && s.recentIssues[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		s.recentIssues = append([]time.Time(nil), s.recentIssues[i:]...)
	}
}

func (s *Store) recentIssueRateLocked() int {
	now := s.now()
	cutoff := now.Add(-loadWindow)
	n := 0
	for _, ts := range s.recentIssues {
		if !ts.Before(cutoff) {
			n++
		}
	}
	return n
}
