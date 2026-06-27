package handlers

import (
	"sync/atomic"

	"github.com/wh-kuromai/allino"
)

var StickyExecutionCount int32

type StickyTestInput struct {
	Value string `query:"value"`
}

type StickyTestOutput struct {
	Result string `json:"result"`
}

var StickyTestHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/stickysessiontest",
		Method:      "GET",
		ContentType: allino.JSON,
		Name:        "sticky-test-handler",
		Version:     "1.0.0",
		Session: allino.SessionOption{
			Type: "sticky",
			Name: "test",
		},
	},
	func(r *allino.Runtime, param StickyTestInput) (*StickyTestOutput, error) {
		atomic.AddInt32(&StickyExecutionCount, 1)
		return &StickyTestOutput{
			Result: "processed-" + param.Value,
		}, nil
	},
)
