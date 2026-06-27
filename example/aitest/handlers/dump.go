package aitest

import (
	"github.com/wh-kuromai/allino"
	"go.uber.org/zap"
)

type DumpAPIOutput struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

var DumpAPI = allino.NewTypedAPI(
	"/api/debug/dump",
	func(r *allino.Runtime, _ any) (*DumpAPIOutput, error) {

		ctx := r.Fiber()

		r.Logger().Debug(
			"http dump",
			zap.Any("method", ctx.Method()),
			zap.Any("path", ctx.Path()),
			zap.Any("query", ctx.Queries()),
			zap.Any("header", ctx.GetReqHeaders()),
			zap.Any("body", string(ctx.Body())),
		)

		return &DumpAPIOutput{
			Method: ctx.Method(),
			Path:   ctx.Path(),
		}, nil
	},
)
