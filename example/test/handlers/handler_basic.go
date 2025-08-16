package handlers

import (
	"errors"
	"time"

	"github.com/wh-kuromai/allino"
)

type EchoAPIInput struct {
	Echo string `query:"echo"`
}
type EchoAPIOutput struct {
	Status  string    `json:"status"`
	Echo    string    `json:"echo,omitempty"`
	StartAt time.Time `json:"startAt"`
}

var EchoAPITypedHandler = allino.NewTypedAPI("/test/echo",
	func(r *allino.Request, param *EchoAPIInput) (*EchoAPIOutput, error) {
		// Actual API logic here.
		return &EchoAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config().StartAt,
		}, nil
	})

type PathParamAPIInput struct {
	Mypath string `path:"mypath"`
	Myform string `form:"myform"`
}
type PathParamAPIOutput struct {
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
	Form   string `json:"form,omitempty"`
}

var PathParamAPITypedHandler = allino.NewTypedAPI("/test/pathparam/:mypath",
	func(r *allino.Request, param *PathParamAPIInput) (*PathParamAPIOutput, error) {
		// Actual API logic here.
		return &PathParamAPIOutput{
			Status: "OK",
			Path:   param.Mypath,
			Form:   param.Myform,
		}, nil
	})

type ValidationAPIInput struct {
	Name  string `query:"name" validate:"required"`       // required なクエリパラメータ
	Email string `form:"email" validate:"required,email"` // required なフォームパラメータ + email形式
}

type ValidationAPIOutput struct {
	Message string `json:"message"`
}

var ValidationAPITypedHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/test/validate",
		Method:      "POST",
		ContentType: "application/json",
		Summary:     "Input validation example",
	},
	func(r *allino.Request, param *ValidationAPIInput) (*ValidationAPIOutput, error) {
		return &ValidationAPIOutput{
			Message: "Valid input received: " + param.Name + " <" + param.Email + ">",
		}, nil
	})

type ErrorTestAPIInput struct {
	Mode string `query:"mode" validate:"required"` // "normal" or "code"
}

type ErrorTestAPIOutput struct {
	Message string `json:"message"`
}

var ErrorTestAPITypedHandler = allino.NewTypedAPI("/test/error",
	func(r *allino.Request, param *ErrorTestAPIInput) (*ErrorTestAPIOutput, error) {
		switch param.Mode {
		case "normal":
			return nil, errors.New("something went wrong")
		case "code":
			return nil, &allino.CodeError{
				StatusCode: 403,
				Code:       "FORBIDDEN",
				Msg:        "you are not allowed",
			}
		default:
			return &ErrorTestAPIOutput{
				Message: "OK",
			}, nil
		}
	})

type RedirectAPIInput struct {
	Target string `query:"target" validate:"required,url"`
}

var RedirectAPITypedHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/test/redirect",
		Method:      "GET",
		ContentType: "text/html",
		Summary:     "Redirect test API",
	},
	func(r *allino.Request, param *RedirectAPIInput) (any, error) {
		return nil, &allino.RedirectError{
			StatusCode: 302,
			Location:   param.Target,
		}
	})
