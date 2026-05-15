package allino_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/wh-kuromai/allino/internal/trie"
)

func TestTrie_ShortestMatch(t *testing.T) {
	trie := trie.NewTrie[string]()

	if err := trie.Insert("/", "root"); err != nil {
		t.Fatal(err)
	}

	if err := trie.Insert("/api", "api"); err != nil {
		t.Fatal(err)
	}

	if err := trie.Insert("/api/users", "users"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		input      string
		wantValue  string
		wantPrefix string
		wantOK     bool
	}{
		{
			name:       "root match",
			input:      "/api/users/123",
			wantValue:  "root",
			wantPrefix: "/",
			wantOK:     true,
		},
		{
			name:       "api match",
			input:      "/api2",
			wantValue:  "root",
			wantPrefix: "/",
			wantOK:     true,
		},
		{
			name:       "exact match",
			input:      "/api/users",
			wantValue:  "root",
			wantPrefix: "/",
			wantOK:     true,
		},
		{
			name:   "not found",
			input:  "xxx",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotPrefix, gotOK := trie.ShortestMatch(tt.input)

			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}

			if gotValue != tt.wantValue {
				t.Fatalf("value = %q, want %q", gotValue, tt.wantValue)
			}

			if gotPrefix != tt.wantPrefix {
				t.Fatalf("prefix = %q, want %q", gotPrefix, tt.wantPrefix)
			}
		})
	}
}

func TestTrie_InsertDuplicate(t *testing.T) {
	trie := trie.NewTrie[int]()

	if err := trie.Insert("/api", 1); err != nil {
		t.Fatal(err)
	}

	if err := trie.Insert("/api", 2); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestTrie_EmptyKey(t *testing.T) {
	trie := trie.NewTrie[int]()

	if err := trie.Insert("", 1); err == nil {
		t.Fatal("expected empty key error")
	}
}

func TestTrie_NodeSplit(t *testing.T) {
	trie := trie.NewTrie[string]()

	if err := trie.Insert("/foobar", "foobar"); err != nil {
		t.Fatal(err)
	}

	if err := trie.Insert("/foobaz", "foobaz"); err != nil {
		t.Fatal(err)
	}

	v1, p1, ok1 := trie.ShortestMatch("/foobar/123")
	if !ok1 {
		t.Fatal("expected match")
	}

	if v1 != "foobar" || p1 != "/foobar" {
		t.Fatalf("got (%q, %q)", v1, p1)
	}

	v2, p2, ok2 := trie.ShortestMatch("/foobaz/123")
	if !ok2 {
		t.Fatal("expected match")
	}

	if v2 != "foobaz" || p2 != "/foobaz" {
		t.Fatalf("got (%q, %q)", v2, p2)
	}
}

func TestTrie_Concurrent(t *testing.T) {
	trie := trie.NewTrie[int]()

	var wg sync.WaitGroup

	// writers
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			key := "/path/" + strconv.Itoa(i)

			_ = trie.Insert(key, i)
		}(i)
	}

	// readers
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			key := "/path/" + strconv.Itoa(i) + "/child"

			trie.ShortestMatch(key)
		}(i)
	}

	wg.Wait()
}
