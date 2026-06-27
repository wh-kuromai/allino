package handlers

import (
	"sync/atomic"
	"time"

	"github.com/wh-kuromai/allino"
)

var AsyncExecutionCount int32

// --------------------
// Async Worker (called side)
// --------------------

type AsyncInput struct {
	Value string
}

type AsyncOutput struct {
	Result string
}

var AsyncWorkerHandler = allino.NewFunction(
	allino.Option{
		Name:    "async-worker",
		Version: "1.0.0",
		JobMode: "async",
	},
	func(r *allino.Runtime, param AsyncInput) (*AsyncOutput, error) {

		atomic.AddInt32(&AsyncExecutionCount, 1)

		// 少し時間かかる処理
		time.Sleep(200 * time.Millisecond)

		return &AsyncOutput{
			Result: "processed-" + param.Value,
		}, nil
	},
)

// --------------------
// Trigger Handler (caller side)
// --------------------

type TriggerInput struct {
	Value string `query:"value"`
}

type TriggerOutput struct {
	Result string `json:"result,omitempty"`
	Status string `json:"status"`
}

var TriggerHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/asynctest",
		Method:      "GET",
		ContentType: allino.JSON,
	},
	func(r *allino.Runtime, param TriggerInput) (*TriggerOutput, error) {

		out, err := AsyncWorkerHandler.Call(r, AsyncInput{
			Value: param.Value,
		})

		if err != nil {
			// ジョブ未完了
			return &TriggerOutput{
				Status: "processing",
			}, nil
		}

		return &TriggerOutput{
			Status: "done",
			Result: out.Result,
		}, nil
	},
)
