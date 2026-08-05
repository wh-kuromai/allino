package allino_test

import (
	"context"
	"testing"

	"github.com/wh-kuromai/allino"
)

func TestJobStoreFacade(t *testing.T) {
	store := s.JobStore()
	if store == nil {
		t.Fatalf("Expected job store to be configured")
	}

	status := store.Status()
	if !status.Configured {
		t.Fatalf("Expected job store status to be configured")
	}
	if status.Backend != "sql" {
		t.Fatalf("Expected sql job backend, got %s", status.Backend)
	}
	if len(status.Handlers) == 0 {
		t.Fatalf("Expected job handlers to be registered")
	}

	if _, err := store.List(context.Background(), allino.JobListFilter{
		Statuses: []string{"queued", "leased", "done", "error"},
		Limit:    10,
	}); err != nil {
		t.Fatalf("Expected job list to work: %v", err)
	}

	if _, err := store.Total(context.Background()); err != nil {
		t.Fatalf("Expected job total to work: %v", err)
	}

	if _, err := store.List(context.Background(), allino.JobListFilter{
		Statuses: []string{"invalid"},
	}); err == nil {
		t.Fatalf("Expected invalid status to fail")
	}
}
