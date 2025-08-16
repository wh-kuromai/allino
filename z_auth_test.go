package allino_test

import (
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/alltest"
	_ "github.com/wh-kuromai/allino/example/test/handlers"
)

func TestAuthCSRF_Success(t *testing.T) {
	form := url.Values{}
	form.Set("message", "hello")

	req := httptest.NewRequest("POST", "/test/authcsrf", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	//h := handlers.AuthCSRFTypedHandler

	// --- ユーザーID をセットしつつ、ログインクッキー＆CSRFトークンも付与 ---
	user := "testuser"
	name := "Test User"
	server := s
	fakeReq := alltest.NewTestRequest(server)
	cookie := allino.IssueLoginCookie(fakeReq, user, name)
	csrfToken := allino.IssueCSRFToken(fakeReq, user)
	req.AddCookie(alltest.FiberToHTTPCookie(cookie))
	req.Header.Set("X-CSRF-Token", csrfToken)

	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Fatalf("Expected 200 OK, got %d", w.StatusCode)
	}
	if !strings.Contains(string(bodybuf), "hello") {
		t.Errorf("Expected echo message in response, got %s", string(bodybuf))
	}
}

func TestAuthCSRF_Unauthorized_NoLogin(t *testing.T) {
	form := url.Values{}
	form.Set("message", "fail")

	req := httptest.NewRequest("POST", "/test/authcsrf", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w, _ := s.Fiber.Test(req, -1)

	if w.StatusCode != 401 {
		t.Errorf("Expected 401 Unauthorized, got %d", w.StatusCode)
	}
}

func TestAuthCSRF_Unauthorized_NoCSRF(t *testing.T) {
	form := url.Values{}
	form.Set("message", "fail")

	req := httptest.NewRequest("POST", "/test/authcsrf", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	//h := handlers.AuthCSRFTypedHandler

	// login cookie はあるが、CSRF token はない
	user := "testuser"
	name := "Test User"
	fakeReq := alltest.NewTestRequest(s)
	cookie := allino.IssueLoginCookie(fakeReq, user, name)
	req.AddCookie(alltest.FiberToHTTPCookie(cookie))

	w, _ := s.Fiber.Test(req, -1)

	if w.StatusCode != 401 {
		t.Errorf("Expected 401 due to missing CSRF token, got %d", w.StatusCode)
	}
}

func TestCORSOptionsResponse(t *testing.T) {
	req := httptest.NewRequest("OPTIONS", "/test/cors", nil)
	w, _ := s.Fiber.Test(req, -1)

	if w.StatusCode != 200 && w.StatusCode != 204 {
		t.Fatalf("Expected 200/204 for OPTIONS, got %d", w.StatusCode)
	}
	if allow := w.Header.Get("Access-Control-Allow-Origin"); allow != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %q", allow)
	}
}
