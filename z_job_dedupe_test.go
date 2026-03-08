package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestJobModeDedupe(t *testing.T) {
	atomic.StoreInt32(&handlers.DedupeExecutionCount, 0)

	var wg sync.WaitGroup
	wg.Add(2)

	results := make([]int, 2)
	errors := make([]error, 2)

	call := func(idx int) {
		defer wg.Done()

		req := httptest.NewRequest("GET", "/api/dedupetest?value=abc", nil)
		resp, _ := s.Fiber.Test(req)
		body, _ := io.ReadAll(resp.Body)

		results[idx] = resp.StatusCode

		if resp.StatusCode != 200 {
			var apiErr allino.APIError[error]
			_ = json.Unmarshal(body, &apiErr)
			errors[idx] = apiErr.Err
		}
	}

	go call(0)
	go call(1)

	wg.Wait()

	// 実行回数は1回のみ
	if atomic.LoadInt32(&handlers.DedupeExecutionCount) != 1 {
		t.Fatalf("handler should execute only once, got %d", handlers.DedupeExecutionCount)
	}

	// 片方は200、片方はエラー
	success := 0
	fail := 0

	for _, code := range results {
		if code == 200 {
			success++
		} else {
			fail++
		}
	}

	if success != 1 || fail != 1 {
		t.Fatalf("expected 1 success and 1 failure, got success=%d fail=%d", success, fail)
	}
}
