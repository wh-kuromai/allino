package allino

import (
	"context"
	"time"

	"github.com/wh-kuromai/allino/internal/timewheel"
)

type TimeWheelConfig struct {
	Slot         int           `json:"slots"`
	TickInterval time.Duration `json:"tick_interval"`
}

func (s *TimeWheelConfig) setup(ctx context.Context) *timewheel.TimeWheel {
	timewheel := timewheel.New(s.Slot, s.TickInterval)
	go timewheel.Run(ctx)

	return timewheel
}
