package handlers

import (
	"sync/atomic"
	"time"

	"github.com/wh-kuromai/allino"
)

var DedupeExecutionCount int32

type DedupeInput struct {
	Value string `query:"value"`
}

type DedupeOutput struct {
	Result string `json:"result"`
}

var DedupeHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/api/dedupetest",
		Method:      "GET",
		ContentType: allino.JSON,
		Name:        "dedupe-test-handler",
		Version:     "1.0.0",
		JobMode:     "dedupe",
	},
	func(r *allino.Request, param DedupeInput) (*DedupeOutput, error) {

		atomic.AddInt32(&DedupeExecutionCount, 1)

		// 同時実行を確実にぶつけるため少し待つ
		time.Sleep(200 * time.Millisecond)

		return &DedupeOutput{
			Result: "processed-" + param.Value,
		}, nil
	},
)
