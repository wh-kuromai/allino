// ping_test.go（test_test パッケージ）

package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestPingHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/ping", nil)
	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)
	//w := httptest.NewRecorder()
	//s.ServeHTTP(w, req)

	if w.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", w.StatusCode)
	}

	var resp allino.APIResponse[handlers.PingOutput]

	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if !resp.Data.RedisOK {
		t.Error("Redis connection failed (RedisOK = false)")
	}
	if !resp.Data.SQLOK {
		t.Error("SQL connection failed (SQLOK = false)")
	}
}
