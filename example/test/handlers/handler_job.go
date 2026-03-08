package handlers

import (
	"sync/atomic"

	"github.com/wh-kuromai/allino"
)

var ExecutionCount int32

type CacheTestInput struct {
	Value string `query:"value"`
}

type CacheTestOutput struct {
	Result string `json:"result"`
}

var CacheTestHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/api/cachetest",
		Method:      "GET",
		ContentType: allino.JSON,
		Name:        "cache-test-handler",
		Version:     "1.0.0",
		JobMode:     "cache",
	},
	func(r *allino.Request, param CacheTestInput) (*CacheTestOutput, error) {
		atomic.AddInt32(&ExecutionCount, 1)
		return &CacheTestOutput{
			Result: "processed-" + param.Value,
		}, nil
	},
)
