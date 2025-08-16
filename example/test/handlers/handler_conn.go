// pingapi.go（test パッケージ）

package handlers

import (
	"github.com/wh-kuromai/allino"
)

type PingOutput struct {
	RedisOK bool `json:"redis"`
	SQLOK   bool `json:"sql"`
}

var PingHandler = allino.NewTypedAPI("/test/ping",
	func(r *allino.Request, _ any) (*PingOutput, error) {
		errRedis := r.Redis().Ping(r.Context()).Err()
		errSQL := r.SQL().PingContext(r.Context())
		return &PingOutput{
			RedisOK: errRedis == nil,
			SQLOK:   errSQL == nil,
		}, nil
	})
