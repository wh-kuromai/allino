package allino

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// 精度を 0.1s (100ms) に設定
const tickInterval = 100 * time.Millisecond

type twTask struct {
	rounds   int
	delay    time.Duration // time.Duration で保持
	fn       func() bool
	canceled atomic.Bool
}

func (t *twTask) Cancel() {
	t.canceled.Store(true)
}

type twWheel struct {
	slots   [][]*twTask
	current int
	mu      sync.Mutex
}

func newTWWheel(slotCount int) *twWheel {
	if slotCount <= 0 {
		slotCount = 32
	}
	return &twWheel{
		slots: make([][]*twTask, slotCount),
	}
}

func (w *twWheel) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval) // 0.1秒ごとに発火
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var tasksToRun []*twTask

			w.mu.Lock()
			tasks := w.slots[w.current]
			w.slots[w.current] = nil

			for _, t := range tasks {
				if t.canceled.Load() {
					continue
				}

				if t.rounds > 0 {
					t.rounds--
					w.slots[w.current] = append(w.slots[w.current], t)
					continue
				}
				tasksToRun = append(tasksToRun, t)
			}

			w.current = (w.current + 1) % len(w.slots)
			w.mu.Unlock()

			// タスクの実行（前回の指摘通りロックの外で実行）
			for _, t := range tasksToRun {
				go func(task *twTask) {
					if task.fn() {
						w.Add(task.delay, task.fn)
					}
				}(t)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (w *twWheel) Add(delay time.Duration, fn func() bool) *twTask {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.addTaskLocked(delay, fn)
}

func (w *twWheel) addTaskLocked(delay time.Duration, fn func() bool) *twTask {
	slotCount := len(w.slots)

	// delay が何 tick 分かを計算 (例: 0.5s / 0.1s = 5 ticks)
	ticks := int(delay / tickInterval)
	if ticks < 1 && delay > 0 {
		ticks = 1 // 最小でも次の tick で実行
	}

	rounds := ticks / slotCount
	idx := (w.current + ticks) % slotCount

	task := &twTask{
		rounds: rounds,
		delay:  delay,
		fn:     fn,
	}

	w.slots[idx] = append(w.slots[idx], task)
	return task
}

func (w *twWheel) Reset(t *twTask, delay time.Duration) {
	t.Cancel()
	w.Add(delay, t.fn)
}
