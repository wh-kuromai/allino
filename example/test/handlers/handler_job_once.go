package handlers

import (
	"sync/atomic"

	"github.com/wh-kuromai/allino"
)

var OnceExecutionCount int32

type OnceInput struct {
	Value string `query:"value"`
}

type OnceOutput struct {
	Result string `json:"result"`
}

var OnceHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/oncetest",
		Method:      "GET",
		ContentType: allino.JSON,
		Name:        "once-test-handler",
		Version:     "1.0.0",
		JobMode:     "once",
	},
	func(r *allino.Runtime, param OnceInput) (*OnceOutput, error) {

		atomic.AddInt32(&OnceExecutionCount, 1)

		return &OnceOutput{
			Result: "processed-" + param.Value,
		}, nil
	},
)
