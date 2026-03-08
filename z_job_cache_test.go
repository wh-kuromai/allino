package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rs/xid"
	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeCache(t *testing.T) {
	atomic.StoreInt32(&handlers.ExecutionCount, 0)
	id := xid.New().String()

	// ---- 1st request ----
	req1 := httptest.NewRequest("GET", "/api/cachetest?value=abc"+id, nil)
	resp1, _ := s.Fiber.Test(req1)
	body1, _ := io.ReadAll(resp1.Body)

	if resp1.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	var out1 allino.APIResponse[handlers.CacheTestOutput]
	if err := json.Unmarshal(body1, &out1); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if out1.Data.Result != ("processed-abc" + id) {
		t.Fatalf("unexpected result: %s", out1.Data.Result)
	}

	// ---- 2nd request (same input, should hit cache) ----
	req2 := httptest.NewRequest("GET", "/api/cachetest?value=abc"+id, nil)
	resp2, _ := s.Fiber.Test(req2)
	body2, _ := io.ReadAll(resp2.Body)

	var out2 allino.APIResponse[handlers.CacheTestOutput]
	_ = json.Unmarshal(body2, &out2)

	if out2.Data.Result != ("processed-abc" + id) {
		t.Fatalf("unexpected cached result: %s", out2.Data.Result)
	}

	if atomic.LoadInt32(&handlers.ExecutionCount) != 1 {
		t.Fatalf("handler should execute only once, got %d", handlers.ExecutionCount)
	}

	// ---- 3rd request (different input → should execute again) ----
	req3 := httptest.NewRequest("GET", "/api/cachetest?value=xyz"+id, nil)
	_, _ = s.Fiber.Test(req3)

	if atomic.LoadInt32(&handlers.ExecutionCount) != 2 {
		t.Fatalf("handler should execute twice after different input, got %d", handlers.ExecutionCount)
	}
}
