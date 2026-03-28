package allino

import "sync"

type jobset struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func newJobset() *jobset {
	return &jobset{
		m: make(map[string]struct{}),
	}
}

func (s *jobset) Add(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[v] = struct{}{}
}

func (s *jobset) Remove(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.m, v)
}

func (s *jobset) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.m)
}

func (s *jobset) Contains(v string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.m[v]
	return ok
}

func (s *jobset) Slice() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r := make([]string, 0, len(s.m))
	for k := range s.m {
		r = append(r, k)
	}
	return r
}

func (s *jobset) Diff(other *jobset) []string {
	if other.Len() == 0 {
		return s.Slice()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, 0, len(s.m))

	for k := range s.m {
		if !other.Contains(k) {
			res = append(res, k)
		}
	}

	return res
}
