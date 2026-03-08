package allino

import (
	"encoding/json"
)

type HttpError interface {
	error
	StatusCode() int
}

type errorcodeError interface {
	error
	ErrorCode() string
}

func NewError(msg string) *Error {
	return &Error{
		Msg: msg,
	}
}

func NewCodeError(status int, code, msg string) *Error {
	return &Error{
		Status: status,
		Code:   code,
		Msg:    msg,
	}
}

func NewCodeErrorWith(status int, code string, err error) *Error {
	return &Error{
		Status: status,
		Code:   code,
		Err:    err,
	}
}

type Error struct {
	Status int    `json:"-"`
	Code   string `json:"code,omitempty"`
	Msg    string `json:"msg,omitempty"`
	Err    error  `json:"-"`
}

func (e *Error) StatusCode() int {
	return e.Status
}

func (e *Error) ErrorCode() string {
	return e.Code
}

func (e *Error) Error() string {
	return e.Msg
}

func (e *Error) With(err error) *Error {
	e.Err = err
	return e
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) JSON() []byte {
	js, err := json.Marshal(e)
	if err != nil {
		return []byte("{}")
	}
	return js
}

func (r *Request) errorRedirect(statusCode int, err error) {
	ecerr, ok2 := err.(errorcodeError)
	if ok2 {
		r.fiber.Set("Location", r.config.Routing.ErrorPath+ecerr.ErrorCode())
	} else {
		r.fiber.Set("Location", r.config.Routing.ErrorPath)
	}

	cerr, ok := err.(HttpError)
	if ok {
		if cerr.StatusCode() != 0 {
			r.fiber.Status(cerr.StatusCode())
		} else {
			r.fiber.Status(statusCode)
		}
	}
}

func (r *Request) errorJSON(statusCode int, nowrap bool, eiserror bool, errz error) {
	var err error
	cerr, ok := errz.(HttpError)
	if ok && cerr.StatusCode() != 0 {
		r.fiber.Status(cerr.StatusCode())
	} else {
		r.fiber.Status(statusCode)
	}

	if nowrap {
		if eiserror {
			buf, erry := json.Marshal(errz)
			if erry != nil || string(buf) == "{}" {
				err = &Error{Msg: errz.Error()}
			} else {
				err = errz
			}
		} else {
			err = errz
		}
	} else {
		if eiserror {
			buf, erry := json.Marshal(errz)
			if erry != nil || string(buf) == "{}" {
				err = &APIError[*Error]{Err: &Error{Msg: errz.Error()}}
			} else {
				err = &APIError[error]{Err: errz}
			}
		} else {
			err = &APIError[error]{Err: errz}
		}
	}
	jerrbuf, err := json.MarshalIndent(err, "", "  ")
	if err == nil {
		r.fiber.Write(jerrbuf)
		return
	}
}
