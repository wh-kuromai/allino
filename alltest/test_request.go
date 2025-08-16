package alltest

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/wh-kuromai/allino"
)

func NewTestRequest(s *allino.Server) *allino.Request {
	req := allino.NewRequest(s, nil)
	return req
}

// 任意の Fiber Cookie を http.Cookie に変換
func FiberToHTTPCookie(fc *fiber.Cookie) *http.Cookie {
	return &http.Cookie{
		Name:     fc.Name,
		Value:    fc.Value,
		Path:     fc.Path,
		Domain:   fc.Domain,
		Expires:  fc.Expires,
		MaxAge:   fc.MaxAge,
		Secure:   fc.Secure,
		HttpOnly: fc.HTTPOnly,
		SameSite: convertSameSite(fc.SameSite),
	}
}

// Fiber SameSite → http.SameSite に変換
func convertSameSite(s string) http.SameSite {
	switch s {
	case "Lax":
		return http.SameSiteLaxMode
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}

/*
func extractParams(pattern, path string) httprouter.Params {
	pParts := strings.Split(strings.Trim(pattern, "/"), "/")
	aParts := strings.Split(strings.Trim(path, "/"), "/")

	params := httprouter.Params{}

	for i := 0; i < len(pParts) && i < len(aParts); i++ {
		if strings.HasPrefix(pParts[i], ":") {
			key := pParts[i][1:]
			params = append(params, httprouter.Param{Key: key, Value: aParts[i]})
		}
	}

	if len(params) == 0 {
		return nil
	}

	return params
}
*/
