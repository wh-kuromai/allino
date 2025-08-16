// htmltemplate_test.go（test_test パッケージ）

package allino_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLTemplateHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/html?name=四葉", nil)
	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", w.StatusCode)
	}
	if !strings.Contains(string(bodybuf), "<h1>Hello, 四葉!") {
		t.Errorf("Expected HTML greeting to contain name, got: %s", bodybuf)
	}
}
