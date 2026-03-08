package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeOnce(t *testing.T) {
	atomic.StoreInt32(&handlers.OnceExecutionCount, 0)

	s := allino.NewTestServer(nil)

	// ---- 1st call (should execute) ----
	req1 := httptest.NewRequest("GET", "/api/oncetest?value=abc", nil)
	resp1, _ := s.Fiber.Test(req1)

	if resp1.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	if atomic.LoadInt32(&handlers.OnceExecutionCount) != 1 {
		t.Fatalf("handler should execute once, got %d", handlers.OnceExecutionCount)
	}

	// ---- 2nd call same input (should be rejected) ----
	req2 := httptest.NewRequest("GET", "/api/oncetest?value=abc", nil)
	resp2, _ := s.Fiber.Test(req2)
	body2, _ := io.ReadAll(resp2.Body)

	if resp2.StatusCode == 200 {
		t.Fatalf("second call should not succeed")
	}

	// optional: error decode check
	var apiErr allino.APIError[error]
	_ = json.Unmarshal(body2, &apiErr)

	if atomic.LoadInt32(&handlers.OnceExecutionCount) != 1 {
		t.Fatalf("handler should still execute only once, got %d", handlers.OnceExecutionCount)
	}

	// ---- 3rd call different input (should execute) ----
	req3 := httptest.NewRequest("GET", "/api/oncetest?value=xyz", nil)
	resp3, _ := s.Fiber.Test(req3)

	if resp3.StatusCode != 200 {
		t.Fatalf("different input should succeed")
	}

	if atomic.LoadInt32(&handlers.OnceExecutionCount) != 2 {
		t.Fatalf("handler should execute twice total, got %d", handlers.OnceExecutionCount)
	}
}
