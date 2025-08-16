package handlers

import (
	"github.com/wh-kuromai/allino"
)

type HTMLTemplateInput struct {
	Name string `query:"name"`
}
type HTMLTemplateOutput struct {
	Message string
}

var HTMLTemplateHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:               "/test/html",
		Method:             "GET",
		ContentType:        "text/html",
		HTMLTemplate:       `<html><body><h1>Hello, {{.Message}}!</h1></body></html>`,
		ResponseStatusCode: 200,
	},
	func(r *allino.Request, param *HTMLTemplateInput) (*HTMLTemplateOutput, error) {
		return &HTMLTemplateOutput{
			Message: param.Name,
		}, nil
	})
