package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/rs/xid"
	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"
)

func TestSession(t *testing.T) {
	id := xid.New().String()

	// ---- 1st request ----
	req1 := httptest.NewRequest("GET", "/api/stickysessiontest?value=abc"+id, nil)

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

}
