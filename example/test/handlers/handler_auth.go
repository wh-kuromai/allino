// authcsrf.go（test パッケージ）

package handlers

import (
	"github.com/wh-kuromai/allino"
)

type AuthCSRFInput struct {
	Message string `form:"message"`
}

type AuthCSRFOutput struct {
	User string `json:"user"`
	Echo string `json:"echo"`
}

var AuthCSRFTypedHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/test/authcsrf",
		Method:      "POST",
		ContentType: "application/json",
		CORS:        true, // CSRF対策の動作確認に必要
		Summary:     "Requires authentication and CSRF token",
	},
	func(r *allino.Request, param *AuthCSRFInput) (*AuthCSRFOutput, error) {
		uid, _, writable, err := r.User()
		if err != nil || !writable {
			return nil, &allino.CodeError{StatusCode: 401, Code: "UNAUTHORIZED", Msg: "login required"}
		}
		return &AuthCSRFOutput{
			User: uid,
			Echo: param.Message,
		}, nil
	})

var CORSHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/test/cors",
		Method:      "POST",
		CORS:        true,
		ContentType: "application/json",
	},
	func(r *allino.Request, _ any) (any, error) {
		return map[string]string{"ok": "yes"}, nil
	})
