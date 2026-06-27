package handlers

import (
	"sync/atomic"
	"time"

	"github.com/wh-kuromai/allino"
)

//
// --------------------
// TTL Worker
// --------------------
//

var TTLExecutionCount int32

type TTLInput struct {
	Value string
}

type TTLOutput struct {
	Result string
}

var TTLWorkerHandler = allino.NewFunction(
	allino.Option{
		Name:    "ttl-worker",
		Version: "1.0.0",
		JobMode: "dispatch",
		Job: allino.JobOption{
			CacheExpire: 2 * time.Second,
		},
	},
	func(r *allino.Runtime, param TTLInput) (*TTLOutput, error) {

		atomic.AddInt32(&TTLExecutionCount, 1)

		return &TTLOutput{
			Result: "ttl-" + param.Value,
		}, nil
	},
)

//
// --------------------
// TTL Trigger API
// --------------------
//

type TTLTriggerInput struct {
	Value string `query:"value"`
}

type TTLTriggerOutput struct {
	Result string `json:"result,omitempty"`
	Status string `json:"status"`
}

var TTLTriggerHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/ttltest",
		Method:      "GET",
		ContentType: allino.JSON,
	},
	func(r *allino.Runtime, param TTLTriggerInput) (*TTLTriggerOutput, error) {

		out, err := TTLWorkerHandler.Call(r, TTLInput{
			Value: param.Value,
		})

		if err != nil {
			return &TTLTriggerOutput{
				Status: "processing",
			}, nil
		}

		return &TTLTriggerOutput{
			Status: "done",
			Result: out.Result,
		}, nil
	},
)

//
// --------------------
// Interval Worker
// --------------------
//

var IntervalExecutionCount int32

type IntervalInput struct {
	Value string
}

type IntervalOutput struct {
	Result string
}

var IntervalWorkerHandler = allino.NewFunction(
	allino.Option{
		Name:    "interval-worker",
		Version: "1.0.0",
		JobMode: "dispatch",
		Job: allino.JobOption{
			Interval: 3 * time.Second,
		},
	},
	func(r *allino.Runtime, param IntervalInput) (*IntervalOutput, error) {

		atomic.AddInt32(&IntervalExecutionCount, 1)

		return &IntervalOutput{
			Result: "interval-" + param.Value,
		}, nil
	},
)

//
// --------------------
// Interval Trigger API
// --------------------
//

type IntervalTriggerInput struct {
	Value string `query:"value"`
}

type IntervalTriggerOutput struct {
	Result string `json:"result,omitempty"`
	Status string `json:"status"`
}

var IntervalTriggerHandler = allino.NewFunction(
	allino.Option{
		Path:        "/api/intervaltest",
		Method:      "GET",
		ContentType: allino.JSON,
	},
	func(r *allino.Runtime, param IntervalTriggerInput) (*IntervalTriggerOutput, error) {

		out, err := IntervalWorkerHandler.Call(r, IntervalInput{
			Value: param.Value,
		})

		if err != nil {
			return &IntervalTriggerOutput{
				Status: "processing",
			}, nil
		}

		return &IntervalTriggerOutput{
			Status: "done",
			Result: out.Result,
		}, nil
	},
)
