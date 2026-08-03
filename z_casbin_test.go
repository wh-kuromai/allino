package allino

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type aclBookInput struct {
	BookID string `path:"book_id" acl:"book"`
}

func TestCasbinACLHTTP(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.conf")
	policyPath := filepath.Join(dir, "policy.csv")

	model := `[request_definition]
r = dom, sub, obj, act

[policy_definition]
p = dom, sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.dom == p.dom && r.sub == p.sub && r.obj == p.obj && r.act == p.act
`
	policy := "p, acme, alice, books/1, read\n"

	if err := os.WriteFile(modelPath, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(&Config{
		Casbin: CasbinConfig{
			ModelPath:  modelPath,
			PolicyPath: policyPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	functionListLen := len(FunctionList)
	fn := NewFunction(
		Option{
			Path:        "/acl/books/:book_id",
			Method:      "GET",
			ContentType: JSON,
			ACLResource: "books/{book}",
			ACLAction:   "read",
		},
		func(r *Runtime, input aclBookInput) (map[string]string, error) {
			return map[string]string{"book": input.BookID}, nil
		},
	)
	defer func() {
		FunctionList = FunctionList[:functionListLen]
	}()
	s.TypedHandle(fn)

	r := NewRuntime(s, nil)
	token := IssueAccessToken(r, "alice", "Alice", map[string]any{"tenant": "acme"})
	req := httptest.NewRequest("GET", "/acl/books/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.Fiber.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCasbinACLDenied(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.conf")
	policyPath := filepath.Join(dir, "policy.csv")

	model := `[request_definition]
r = dom, sub, obj, act

[policy_definition]
p = dom, sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.dom == p.dom && r.sub == p.sub && r.obj == p.obj && r.act == p.act
`
	policy := "p, acme, alice, books/1, read\n"

	if err := os.WriteFile(modelPath, []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(&Config{
		Casbin: CasbinConfig{
			ModelPath:  modelPath,
			PolicyPath: policyPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	functionListLen := len(FunctionList)
	fn := NewFunction(
		Option{
			Path:        "/acl-denied/books/:book_id",
			Method:      "GET",
			ContentType: JSON,
			ACLResource: "books/:book",
			ACLAction:   "read",
		},
		func(r *Runtime, input aclBookInput) (map[string]string, error) {
			return map[string]string{"book": input.BookID}, nil
		},
	)
	defer func() {
		FunctionList = FunctionList[:functionListLen]
	}()
	s.TypedHandle(fn)

	r := NewRuntime(s, nil)
	token := IssueAccessToken(r, "bob", "Bob", map[string]any{"tenant": "acme"})
	req := httptest.NewRequest("GET", "/acl-denied/books/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.Fiber.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
