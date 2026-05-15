package allino_test

import (
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeFanout(t *testing.T) {
	atomic.StoreInt32(&handlers.FanoutExecutionCount, 0)

	//s := allino.NewTestServer(nil)

	// ---- 1st call (should execute) ----

	req1 := httptest.NewRequest("GET", "/api/fanouttest?value=abc", nil)
	resp1, eee := s.Fiber.Test(req1)

	fmt.Println("TestJobModeFanout", resp1, eee)

	time.Sleep(time.Second)

	if resp1.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp1.StatusCode)
	}

	if atomic.LoadInt32(&handlers.FanoutExecutionCount) != 1 {
		t.Fatalf("handler should execute once, got %d", handlers.FanoutExecutionCount)
	}

}

func TestJobModeReplay(t *testing.T) {
	atomic.StoreInt32(&handlers.ReplayExecutionCount, 0)

	//s := allino.NewTestServer(nil)

	// ---- 1st call (should execute) ----
	req1 := httptest.NewRequest("GET", "/api/replaytest?value=abc", nil)
	resp1, eee := s.Fiber.Test(req1)

	fmt.Println("TestJobModeReplay", resp1, eee)

	time.Sleep(time.Second)

	if resp1.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp1.StatusCode)
	}

	if atomic.LoadInt32(&handlers.ReplayExecutionCount) != 1 {
		t.Fatalf("handler should execute once, got %d", handlers.ReplayExecutionCount)
	}

}

func TestJobModeReplayAll(t *testing.T) {
	atomic.StoreInt32(&handlers.ReplayAllExecutionCount, 0)

	//s := allino.NewTestServer(nil)

	// ---- 1st call (should execute) ----
	req1 := httptest.NewRequest("GET", "/api/replayalltest?value=abc", nil)
	resp1, eee := s.Fiber.Test(req1)

	fmt.Println("TestJobModeReplayAll", resp1, eee)

	time.Sleep(time.Second)

	if resp1.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp1.StatusCode)
	}

	if atomic.LoadInt32(&handlers.ReplayAllExecutionCount) != 1 {
		t.Fatalf("handler should execute once, got %d", handlers.ReplayAllExecutionCount)
	}

}
