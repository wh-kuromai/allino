package trie

import (
	"errors"
	"sync"
)

var (
	ErrEmptyKey  = errors.New("empty key")
	ErrNotFound  = errors.New("no match found")
	ErrDuplicate = errors.New("key already exists")
)

type Trie[T any] struct {
	mu   sync.RWMutex
	root *node[T]
}

type node[T any] struct {
	prefix   string
	children []*node[T]

	hasValue bool
	value    T
}

func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{
		root: &node[T]{},
	}
}

// Insert inserts a key/value pair.
//
// Example:
//
//	trie.Insert("/api/users", value)
func (t *Trie[T]) Insert(key string, value T) error {
	if key == "" {
		return ErrEmptyKey
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.root.insert(key, value)
}

// MatchAll walks all matching prefixes in order from shortest to longest.
//
// callback returns false to stop iteration early.
func (t *Trie[T]) MatchAll(
	s string,
	fn func(value T, matchedPrefix string) bool,
) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	n := t.root
	search := s
	matched := ""

	for {
		if n.hasValue {
			if fn(n.value, matched) {
				return true
			}
		}

		found := false

		for _, child := range n.children {
			if hasPrefix(search, child.prefix) {
				search = search[len(child.prefix):]
				matched += child.prefix
				n = child
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}
}

func (t *Trie[T]) ShortestMatch(s string) (value T, matchedPrefix string, ok bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var zero T

	n := t.root
	search := s
	matched := ""

	for {
		// shortest match
		if n.hasValue {
			return n.value, matched, true
		}

		found := false

		for _, child := range n.children {
			if hasPrefix(search, child.prefix) {
				search = search[len(child.prefix):]
				matched += child.prefix
				n = child
				found = true
				break
			}
		}

		if !found {
			return zero, "", false
		}
	}
}

func (n *node[T]) insert(key string, value T) error {
	for _, child := range n.children {
		common := longestCommonPrefix(key, child.prefix)

		if common == 0 {
			continue
		}

		// exact match
		if common == len(child.prefix) && common == len(key) {
			if child.hasValue {
				return ErrDuplicate
			}

			child.hasValue = true
			child.value = value
			return nil
		}

		// descend
		if common == len(child.prefix) {
			return child.insert(key[common:], value)
		}

		// split node
		split := &node[T]{
			prefix:   child.prefix[common:],
			children: []*node[T]{
				// preserve grandchildren later
			},
			hasValue: child.hasValue,
			value:    child.value,
		}

		split.children = child.children

		// rewrite current child as parent
		child.prefix = child.prefix[:common]
		child.children = []*node[T]{split}
		child.hasValue = false

		var zero T
		child.value = zero

		remaining := key[common:]

		if remaining == "" {
			child.hasValue = true
			child.value = value
			return nil
		}

		child.children = append(child.children, &node[T]{
			prefix:   remaining,
			hasValue: true,
			value:    value,
		})

		return nil
	}

	// new leaf
	n.children = append(n.children, &node[T]{
		prefix:   key,
		hasValue: true,
		value:    value,
	})

	return nil
}

func longestCommonPrefix(a, b string) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}

	i := 0
	for i < max && a[i] == b[i] {
		i++
	}

	return i
}

func hasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}

	return s[:len(prefix)] == prefix
}
