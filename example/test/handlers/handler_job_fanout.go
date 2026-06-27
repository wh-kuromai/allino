package handlers

import (
	"sync/atomic"

	"github.com/wh-kuromai/allino"
)

var FanoutExecutionCount int32

type FanoutInput struct {
	Value string `query:"value"`
}

type FanoutOutput struct {
}

var FanoutHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/fanouttest",
		ContentType: allino.JSON,
		Name:        "fanout-test-handler",
		Version:     "1.0.0",
		JobMode:     "fanout",
	},
	func(r *allino.Runtime, param *FanoutInput) (*FanoutOutput, error) {
		atomic.AddInt32(&FanoutExecutionCount, 1)
		r.Logger().Info("/api/fanouttest")
		return nil, nil
	},
)

var ReplayExecutionCount int32

var ReplayHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/replaytest",
		ContentType: allino.JSON,
		Name:        "replay-test-handler",
		Version:     "1.0.0",
		JobMode:     "replay",
	},
	func(r *allino.Runtime, param *FanoutInput) (*FanoutOutput, error) {
		atomic.AddInt32(&ReplayExecutionCount, 1)
		return nil, nil
	},
)

var ReplayAllExecutionCount int32

var ReplayAllHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/replayalltest",
		ContentType: allino.JSON,
		Name:        "replayall-test-handler",
		Version:     "1.0.0",
		JobMode:     "replayall",
	},
	func(r *allino.Runtime, param *FanoutInput) (*FanoutOutput, error) {
		atomic.AddInt32(&ReplayAllExecutionCount, 1)
		return nil, nil
	},
)
