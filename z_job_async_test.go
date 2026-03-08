package allino_test

import (
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/xid"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeAsync(t *testing.T) {
	id := xid.New().String()
	atomic.StoreInt32(&handlers.AsyncExecutionCount, 0)

	// ---- 1st call: enqueue ----
	req1 := httptest.NewRequest("GET", "/api/asynctest?value=abc"+id, nil)
	resp1, _ := s.Fiber.Test(req1)

	if resp1.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	// まだ実行されていない可能性あり
	if atomic.LoadInt32(&handlers.AsyncExecutionCount) != 0 {
		t.Fatalf("async worker should not run immediately")
	}

	// ---- wait for worker ----
	time.Sleep(2 * time.Second)

	// 実行されているはず
	if atomic.LoadInt32(&handlers.AsyncExecutionCount) != 1 {
		t.Fatalf("async worker should run once, got %d", handlers.AsyncExecutionCount)
	}

	// ---- 2nd call: should return result ----
	req2 := httptest.NewRequest("GET", "/api/asynctest?value=abc"+id, nil)
	resp2, _ := s.Fiber.Test(req2)

	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200 on second call")
	}

	// まだ実行されていない可能性あり
	if atomic.LoadInt32(&handlers.AsyncExecutionCount) != 1 {
		t.Fatalf("async worker should not run immediately")
	}

	// ---- wait for worker ----
	time.Sleep(2 * time.Second)

	// 実行されているはず
	if atomic.LoadInt32(&handlers.AsyncExecutionCount) != 2 {
		t.Fatalf("async worker should run twice, got %d", handlers.AsyncExecutionCount)
	}
}
