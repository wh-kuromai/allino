package allino_test

import (
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/xid"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeTTL(t *testing.T) {
	id := xid.New().String()
	atomic.StoreInt32(&handlers.TTLExecutionCount, 0)

	req1 := httptest.NewRequest("GET", "/api/ttltest?value=abc"+id, nil)
	resp1, _ := s.Fiber.Test(req1)

	if resp1.StatusCode != 200 {
		t.Fatalf("expected 200")
	}

	time.Sleep(1200 * time.Millisecond)

	if atomic.LoadInt32(&handlers.TTLExecutionCount) != 1 {
		t.Fatalf("worker should run once")
	}

	// TTL内
	req2 := httptest.NewRequest("GET", "/api/ttltest?value=abc"+id, nil)
	s.Fiber.Test(req2)

	time.Sleep(300 * time.Millisecond)

	if atomic.LoadInt32(&handlers.TTLExecutionCount) != 1 {
		t.Fatalf("should use cached result before TTL expires")
	}

	// TTL後
	time.Sleep(2 * time.Second)

	req3 := httptest.NewRequest("GET", "/api/ttltest?value=abc"+id, nil)
	s.Fiber.Test(req3)

	time.Sleep(2 * time.Second)

	if atomic.LoadInt32(&handlers.TTLExecutionCount) != 2 {
		t.Fatalf("worker should run again after TTL expiration: %d", handlers.TTLExecutionCount)
	}
}

func TestJobModeInterval(t *testing.T) {
	atomic.StoreInt32(&handlers.IntervalExecutionCount, 0)
	id := xid.New().String()

	req1 := httptest.NewRequest("GET", "/api/intervaltest?value=abc"+id, nil)
	s.Fiber.Test(req1)

	time.Sleep(time.Second)

	if atomic.LoadInt32(&handlers.IntervalExecutionCount) != 1 {
		t.Fatalf("worker should run once: %d", handlers.IntervalExecutionCount)
	}

	// Interval内
	id = xid.New().String()
	req2 := httptest.NewRequest("GET", "/api/intervaltest?value=abc"+id, nil)
	s.Fiber.Test(req2)

	time.Sleep(time.Second)

	if atomic.LoadInt32(&handlers.IntervalExecutionCount) != 1 {
		t.Fatalf("worker should not run again within interval: %d", handlers.IntervalExecutionCount)
	}

	// Interval後
	time.Sleep(5 * time.Second)

	id = xid.New().String()
	req3 := httptest.NewRequest("GET", "/api/intervaltest?value=abc"+id, nil)
	s.Fiber.Test(req3)

	time.Sleep(time.Second)

	if atomic.LoadInt32(&handlers.IntervalExecutionCount) != 3 {
		t.Fatalf("worker should run again after interval: %d", handlers.IntervalExecutionCount)
	}
}
