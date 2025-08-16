package handlers

import (
	"time"

	"github.com/wh-kuromai/allino"
)

type HealthcheckAPIInput struct {
	Echo string `query:"echo"`
}
type HealthcheckAPIOutput struct {
	Status  string    `json:"status"`
	Echo    string    `json:"echo,omitempty"`
	StartAt time.Time `json:"startAt"`
}

var HealthcheckAPITypedHandler = allino.NewTypedAPI("/api/healthcheck",
	func(r *allino.Request, param HealthcheckAPIInput) (HealthcheckAPIOutput, error) {
		// Actual API logic here.
		return HealthcheckAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config().StartAt,
		}, nil
	})

var HealthcheckBothAPITypedHandler = allino.NewTypedAPI("/api/healthcheck_both",
	func(r *allino.Request, param HealthcheckAPIInput) (*HealthcheckAPIOutput, error) {
		// Actual API logic here.
		return &HealthcheckAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config().StartAt,
		}, nil
	})

var HealthcheckPointerAPITypedHandler = allino.NewTypedAPI("/api/healthcheck_pointer",
	func(r *allino.Request, param *HealthcheckAPIInput) (*HealthcheckAPIOutput, error) {
		// Actual API logic here.
		return &HealthcheckAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config().StartAt,
		}, nil
	})
