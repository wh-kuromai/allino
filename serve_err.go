package allino

import (
	"encoding/json"
)

type CodeError struct {
	StatusCode int          `json:"-"`
	Code       string       `json:"code,omitempty"`
	Msg        string       `json:"msg,omitempty"`
	Err        error        `json:"-"`
	Child      []*CodeError `json:"child,omitempty"`
}

func (e CodeError) Error() string {
	return "[" + string(e.Code) + "] " + e.Msg
}

func (r *Request) errorRedirect(statusCode int, err error) {
	cerr, ok := err.(*CodeError)
	if ok {
		r.fiber.Set("Location", r.config.Routing.ErrorPath+cerr.Code)
		if cerr.StatusCode != 0 {
			r.fiber.Status(cerr.StatusCode)
		} else {
			r.fiber.Status(statusCode)
		}
		return
	}

	r.fiber.Set("Location", r.config.Routing.ErrorPath)
	r.fiber.Status(statusCode)
}

func (r *Request) errorJSON(statusCode int, nowrap bool, err error) {
	cerr, ok := err.(*CodeError)
	if ok {
		if cerr.StatusCode != 0 {
			r.fiber.Status(cerr.StatusCode)
		} else {
			r.fiber.Status(statusCode)
		}

		if nowrap {
			err = cerr
		} else {
			err = &APIError[*CodeError]{Err: cerr}
		}
		jerrbuf, err := json.MarshalIndent(err, "", "  ")
		if err == nil {
			r.fiber.Write(jerrbuf)
			return
		}
	}
	r.fiber.Status(statusCode)

	if nowrap {
		err = &Error{Msg: err.Error()}
	} else {
		err = &APIError[*Error]{Err: &Error{Msg: err.Error()}}
	}
	jerrbuf, err := json.MarshalIndent(err, "", "  ")
	if err == nil {
		r.fiber.Write(jerrbuf)
		return
	}
}
