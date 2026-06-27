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

var HTMLTemplateHandler = allino.NewFunction(
	allino.Option{
		Path:               "/test/html",
		Method:             "GET",
		ContentType:        "text/html",
		HTMLTemplate:       `<html><body><h1>Hello, {{.Message}}!</h1></body></html>`,
		ResponseStatusCode: 200,
	},
	func(r *allino.Runtime, param *HTMLTemplateInput) (*HTMLTemplateOutput, error) {
		return &HTMLTemplateOutput{
			Message: param.Name,
		}, nil
	})
