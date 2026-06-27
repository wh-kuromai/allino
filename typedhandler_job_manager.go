package allino

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wh-kuromai/allino/internal/ema"
	"go.uber.org/zap"
)

type jobProgress struct {
	Current int
	Total   int
}

type jobManager struct {
	handlers *jobset
	//handlerOptMap        map[string]*Option
	lockedHandlers         *jobset
	resourcelockedHandlers *jobset
	dequeueThroughputEMA   *ema.EMACalculator

	activeJobs int64 // 実行中ジョブ数
	attempt    int64 // dequeue 失敗回数

	waiting atomic.Bool
	doneCh  chan struct{} // 完了通知

	inited  sync.Once
	queueCh chan func()
}

func newJobManager() *jobManager {
	return &jobManager{
		handlers: newJobset(),
		//handlerOptMap:        make(map[string]*Option),
		lockedHandlers:         newJobset(),
		resourcelockedHandlers: newJobset(),
		dequeueThroughputEMA:   ema.NewEMACalculator(0.3),
		doneCh:                 make(chan struct{}),
		queueCh:                make(chan func()),
	}
}

func (jobm *jobManager) Init(sv *Server) {
	jobm.inited.Do(func() {
		numgoroutine := sv.Config.JobConfig.Concurrency
		for i := 0; i < numgoroutine; i++ {
			go func() {
				for {
					select {
					case <-sv.appctx.Done():
						return
					case fn := <-jobm.queueCh:
						fn()
					}
				}
			}()
		}
	})
}

func (jobm *jobManager) Do(fn func()) {
	jobm.queueCh <- fn
}

func (jobm *jobManager) WorkerInit(s callStrategy, sv *Server) {
	jobm.Init(sv)
	//idleit := sv.Config.JobConfig.IdleInterval
	//leasesec := int(mergeSingle(sv.Config.JobConfig.LeaseSeconds, opt.Job.LeaseSeconds).Seconds())

	leaset := sv.Config.JobConfig.LeaseDuration

	//jobchan := make(chan JobTask, sv.Config.JobConfig.Concurrency*2)

	backoff := NewBackoff(100*time.Millisecond, sv.Config.JobConfig.IdleInterval)

	go func() {
		for {
			select {
			case <-sv.appctx.Done():
				return
			default:
				hs := jobm.handlers.Diff(jobm.lockedHandlers, jobm.resourcelockedHandlers)
				jtask, err := s.Dequeue(sv.appctx, hs, leaset, jobm.dequeueThroughputEMA)
				if err != nil {
					if !errors.Is(err, ErrJobNotFound) {
						sv.Logger.Error("job system error", zap.String("component", "dequeue"), zap.Error(err))
					}

					wait := backoff.Duration(int(atomic.AddInt64(&jobm.attempt, 1)))
					if jobm.waiting.Load() {
						jobm.tryFinish()
					}
					time.Sleep(wait)
					continue
				}

				jtask.Meta().ParentID = jtask.Key()
				if jtask.Meta().RootID == "" {
					jtask.Meta().RootID = jtask.Key()
				}

				// if dequeued need handler lock,
				opt := sv.handlerOptMap[jtask.Handler()]
				if opt.Job.Interval != 0 {
					jobm.lockedHandlers.Add(jtask.Handler())
				}

				atomic.StoreInt64(&jobm.attempt, 0)
				//jobchan <- jtask

				jobm.Do(func() {

					now := time.Now()
					opt := sv.handlerOptMap[jtask.Handler()]

					// check if need to re-dispatch
					needDispatch := false
					var waitDur time.Duration
					if opt.Job.Interval != 0 && opt.lastRun != nil {
						waitDur = time.Until((*opt.lastRun).Add(time.Duration(float64(opt.Job.Interval) * jobm.dequeueThroughputEMA.CurrentAverage)))
						if waitDur > 0 {
							needDispatch = true
						}
					}

					task := sv.TimeWheel.Add(time.Duration(leaset/2), func() bool {
						err := jtask.HeartBeat(sv.appctx, leaset)
						//err := s.LeaseUpdate(sv.appctx, jobn.key, leaset)
						if err != nil && !sv.Config.Log.Silent {
							sv.Logger.Error("job system error", zap.String("component", "leaseupdate"), zap.Error(err))
						}
						return true
					})

					atomic.AddInt64(&jobm.activeJobs, 1)
					fn := func() bool {
						defer func() {
							atomic.AddInt64(&jobm.activeJobs, -1)
							if jobm.waiting.Load() {

								jobm.tryFinish()
							}
						}()

						r := NewRequest(sv, nil)
						defer r.do_defer()
						r.cache.requestid = jtask.Key()
						r.cache.req_type = REQUEST_JOB
						r.cache.parentjobid = jtask.Meta().ParentID
						r.cache.rootjobid = jtask.Meta().RootID

						if !r.config.Log.Silent {
							r.logger.Info("job started", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
						}
						_, outjson, errjson, syserr := opt.consumer(r, jtask.Handler(), jtask.Meta().Version, jtask.Input(), false, nil)
						if syserr != nil {
							if !r.config.Log.Silent {
								r.logger.Error("job failed", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()), zap.Error(syserr))
							}

							err := jtask.Fail(sv.appctx)
							//err := s.Free(sv.appctx, jtask.Key())
							if err != nil && !r.config.Log.Silent {
								r.logger.Error("job system error", zap.String("component", "dequeue/syserr"), zap.Error(err))
							}
						} else if r.memo.jobabortctrl != "" {
							// cancel if abort or error
							if !r.config.Log.Silent {
								r.logger.Info("job requeued", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Requeue(sv.appctx, requeueDelay(r))
							//err := s.Requeue(sv.appctx, jtask.Key(), requeueDelay(r))
							if err != nil && !r.config.Log.Silent {
								r.logger.Error("job system error", zap.String("component", "dequeue/requeue"), zap.Error(err))
							}
						} else if opt.Job.Cache {

							//var ttl *time.Time
							if opt.Job.CacheExpire != 0 {
								ttlb := time.Now().Add(opt.Job.CacheExpire)
								//ttl = &ttlb

								jtask.Meta().TTL = &ttlb
							}

							// store if cache or dedupe
							if !r.config.Log.Silent {
								r.logger.Info("job completed/cached", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Success(sv.appctx, jtask.Handler(), jtask.Meta(), jtask.Key(), jtask.Input(), outjson, errjson)
							//err := s.DoneAsync(sv.appctx, jtask.Key(), ttl, outjson, errjson)
							if err != nil && !r.config.Log.Silent {
								r.logger.Error("job system error", zap.String("component", "dequeue/doneasync"), zap.Error(err))
							}
						} else {
							if !r.config.Log.Silent {
								r.logger.Info("job completed", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Fail(sv.appctx)
							//err := s.Free(sv.appctx, jtask.Key())
							if err != nil && !r.config.Log.Silent {
								r.logger.Error("job system error", zap.String("component", "dequeue/etc"), zap.Error(err))
							}
						}
						task.Cancel()
						if needDispatch {
							jobm.lockedHandlers.Remove(jtask.Handler())
							opt.lastRun = &now
						}
						return false
					}

					if needDispatch {
						sv.TimeWheel.Add(waitDur, fn)
					} else {
						fn()
						jobm.lockedHandlers.Remove(jtask.Handler())
						opt.lastRun = &now
					}
				})
			}
		}
	}()

}

func (jobm *jobManager) tryFinish() {
	if atomic.LoadInt64(&jobm.activeJobs) == 0 &&
		atomic.LoadInt64(&jobm.attempt) >= 5 {

		select {
		case jobm.doneCh <- struct{}{}:
		default:
		}
	}
}

func (jobm *jobManager) WaitForJob(ctx context.Context, c callStrategy, jobid string, progressf func(doneCount, errCount, total int)) error {
	jobm.waiting.Store(true)

	ticker := time.NewTicker(1 * time.Second)
	tickerfn := func() {
		statusCountMap, err := c.Total(ctx, "*")
		if err == nil {
			if progressf != nil {
				total := 0
				for _, v := range statusCountMap {
					total += v
				}
				progressf(statusCountMap["done"], statusCountMap["error"], total)
			}
		}
	}
	defer ticker.Stop()
	for {
		select {
		case <-jobm.doneCh:
			tickerfn()
			jobm.waiting.Store(false)
			return nil
		case <-ctx.Done():
			tickerfn()
			return ctx.Err()
		case <-ticker.C:
			// 👇ここで定期処理
			tickerfn()
		}
	}
}
