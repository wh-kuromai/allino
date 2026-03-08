package allino

type jobset map[string]struct{}

func newJobset() jobset {
	return make(map[string]struct{})
}

func (s jobset) Add(v string) {
	s[v] = struct{}{}
}

func (s jobset) Remove(v string) {
	delete(s, v)
}

func (s jobset) Len() int {
	return len(s)
}

func (s jobset) Contains(v string) bool {
	_, ok := s[v]
	return ok
}

func (s jobset) Slice() []string {
	r := make([]string, 0, len(s))
	for k := range s {
		r = append(r, k)
	}
	return r
}

func (s jobset) Diff(other jobset) []string {
	if other.Len() == 0 {
		return s.Slice()
	}

	res := make([]string, 0, len(s))

	for k := range s {
		if !other.Contains(k) {
			res = append(res, k)
		}
	}

	return res
}
