package allino

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	JOBMODE_NORMAL   = ""
	JOBMODE_ASYNC    = "async"
	JOBMODE_CACHE    = "cache"
	JOBMODE_DEDUPE   = "dedupe"
	JOBMODE_ONCE     = "once"
	JOBMODE_MEMOIZED = "memoized"
	JOBMODE_DISPATCH = "dispatch"

	//JOB_BACKEND_SQL = "sql"
	//JOB_BACKEND_REDIS = "backend_redis"
)

type JobConfig struct {
	//Backend         string        `json:"backend"`
	MaxRetry        int           `json:"max_retry"`
	IdleInterval    time.Duration `json:"idle_interval"`
	Concurrency     int           `json:"concurrency"`
	LeaseDuration   time.Duration `json:"lease_duration"`
	RequeueInterval time.Duration `json:"requeue_interval"`
	WaitTimeout     time.Duration `json:"wait_timeout"`
	WaitInterval    time.Duration `json:"wait_interval"`

	RedisKeyPrefix   string `json:"redis_key_prefix"`
	RedisStreamGroup string `json:"redis_stream_group"`
}

type JobOption struct {
	Async         bool
	Cache         bool
	CacheErrOnHit bool // used by JOBMODE_ONCE
	Dedupe        bool
	Priority      int
	CacheExpire   time.Duration
	Interval      time.Duration

	//Backend string

	callstrategy callStrategy
}

type JobConf struct {
}

type JobOpt struct {
}

type jobManager struct {
	handlers             *jobset
	handlerOptMap        map[string]*HandlerOption
	lockedHandlers       *jobset
	dequeueThroughputEMA *EMACalculator
}

var JobExtension = NewExtension[JobConf, JobOpt](
	"job",
	&ExtOption{
		OnHandlerInit: func(s *Server, virtual *Request, opt *HandlerOption) error {

			switch opt.JobMode {
			case JOBMODE_ASYNC:
				opt.Job.Async = true
			case JOBMODE_CACHE:
				opt.Job.Cache = true
			case JOBMODE_DEDUPE:
				opt.Job.Dedupe = true
			case JOBMODE_ONCE:
				opt.Job.Dedupe = true
				opt.Job.Cache = true
				opt.Job.CacheErrOnHit = true
			case JOBMODE_MEMOIZED:
				opt.Job.Dedupe = true
				opt.Job.Cache = true
			case JOBMODE_DISPATCH:
				opt.Job.Async = true
				opt.Job.Cache = true
			}

			if opt.JobMode != "" {
				//hbackend := mergeSingle(s.Config.JobConfig.Backend, opt.Job.Backend)
				if s.jobManagerCache == nil {
					s.jobManagerCache = &jobManager{
						handlers:             newJobset(),
						handlerOptMap:        make(map[string]*HandlerOption),
						lockedHandlers:       newJobset(),
						dequeueThroughputEMA: NewEMACalculator(0.3),
					}
				}

				if callSQLstrategy == nil {
					callSQLstrategy = newcallSQLStrategy(s)
					err := callSQLstrategy.Init(virtual.Context())
					if err != nil && !s.Config.Log.Silent {
						s.Logger.Info("job system error", zap.String("component", "register"), zap.Error(err))
					}
					strategyWorkerInit(callSQLstrategy, s)
					s.initTaskWheel()

					// Reaping
					s.taskwheel.Add(s.Config.JobConfig.LeaseDuration, func() bool {
						err = callSQLstrategy.Reaping(s.appctx)
						if err != nil && !s.Config.Log.Silent {
							s.Logger.Info("job system error", zap.String("component", "reaping"), zap.Error(err))
						}
						return true
					})
				}

				opt.Job.callstrategy = callSQLstrategy
				s.jobManagerCache.handlers.Add(encodeHandlerName(opt))
				s.jobManagerCache.handlerOptMap[encodeHandlerName(opt)] = opt
			}

			return nil
		},

		OnShutdown: func(s *Server, virtual *Request) error {
			return nil
		},
	},
)

func strategyWorkerInit(s callStrategy, sv *Server) {
	//idleit := sv.Config.JobConfig.IdleInterval
	//leasesec := int(mergeSingle(sv.Config.JobConfig.LeaseSeconds, opt.Job.LeaseSeconds).Seconds())

	leaset := sv.Config.JobConfig.LeaseDuration

	jobchan := make(chan JobTask, sv.Config.JobConfig.Concurrency*2)

	backoff := NewBackoff(100*time.Millisecond, sv.Config.JobConfig.IdleInterval)
	attempt := 0

	go func() {
		for {
			select {
			case <-sv.appctx.Done():
				return
			default:
				hs := sv.jobManagerCache.handlers.Diff(sv.jobManagerCache.lockedHandlers)
				jtask, err := s.Dequeue(sv.appctx, hs, leaset, sv.jobManagerCache.dequeueThroughputEMA)
				if err != nil {
					if !errors.Is(err, ErrJobNotFound) {
						sv.Logger.Error("job system error", zap.String("component", "dequeue"), zap.Error(err))
					}

					attempt++
					wait := backoff.Duration(attempt)
					time.Sleep(wait)
					continue
				}

				jtask.Meta().ParentID = jtask.Key()
				if jtask.Meta().RootID == "" {
					jtask.Meta().RootID = jtask.Key()
				}

				// if dequeued need handler lock,
				opt := sv.jobManagerCache.handlerOptMap[jtask.Handler()]
				if opt.Job.Interval != 0 {
					sv.jobManagerCache.lockedHandlers.Add(jtask.Handler())
				}

				attempt = 0
				jobchan <- jtask
			}
		}
	}()

	numgoroutine := sv.Config.JobConfig.Concurrency
	for i := 0; i < numgoroutine; i++ {
		go func() {
			for {
				select {
				case <-sv.appctx.Done():
					return
				case jtask := <-jobchan:
					now := time.Now()
					opt := sv.jobManagerCache.handlerOptMap[jtask.Handler()]

					// check if need to re-dispatch
					needDispatch := false
					var waitDur time.Duration
					if opt.Job.Interval != 0 && opt.lastRun != nil {
						waitDur = time.Until((*opt.lastRun).Add(time.Duration(float64(opt.Job.Interval) * sv.jobManagerCache.dequeueThroughputEMA.CurrentAverage)))
						if waitDur > 0 {
							needDispatch = true
						}
					}

					task := sv.taskwheel.Add(time.Duration(leaset/2), func() bool {
						err := jtask.HeartBeat(sv.appctx, leaset)
						//err := s.LeaseUpdate(sv.appctx, jobn.key, leaset)
						if err != nil && !sv.Config.Log.Silent {
							sv.Logger.Info("job system error", zap.String("component", "leaseupdate"), zap.Error(err))
						}
						return true
					})

					fn := func() bool {
						r := NewRequest(sv, nil)
						r.cache.req_type = REQUEST_JOB
						r.cache.requestid = jtask.Key()
						r.cache.parentjobid = jtask.Meta().ParentID
						r.cache.rootjobid = jtask.Meta().RootID

						if !r.config.Log.Silent {
							r.logger.Info("job started", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
						}
						_, outjson, errjson, syserr := opt.consumer(r, jtask.Handler(), jtask.Input())
						if syserr != nil {
							if !r.config.Log.Silent {
								r.logger.Info("job failed", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()), zap.Error(syserr))
							}

							err := jtask.Fail(sv.appctx)
							//err := s.Free(sv.appctx, jtask.Key())
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/syserr"), zap.Error(err))
							}
						} else if r.memo.jobabortctrl != "" {
							// cancel if abort or error
							if !r.config.Log.Silent {
								r.logger.Info("job requeued", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Requeue(sv.appctx, requeueDelay(r))
							//err := s.Requeue(sv.appctx, jtask.Key(), requeueDelay(r))
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/requeue"), zap.Error(err))
							}
						} else if opt.Job.Cache {

							var ttl *time.Time
							if opt.Job.CacheExpire != 0 {
								ttlb := time.Now().Add(opt.Job.CacheExpire)
								ttl = &ttlb
							}

							// store if cache or dedupe
							if !r.config.Log.Silent {
								r.logger.Info("job completed (cache)", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Success(sv.appctx, ttl, outjson, errjson)
							//err := s.DoneAsync(sv.appctx, jtask.Key(), ttl, outjson, errjson)
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/doneasync"), zap.Error(err))
							}
						} else {
							if !r.config.Log.Silent {
								r.logger.Info("job completed", zap.String("handler", jtask.Handler()), zap.String("requestid", jtask.Key()))
							}

							err := jtask.Fail(sv.appctx)
							//err := s.Free(sv.appctx, jtask.Key())
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/etc"), zap.Error(err))
							}
						}
						task.Cancel()
						if needDispatch {
							sv.jobManagerCache.lockedHandlers.Remove(jtask.Handler())
							opt.lastRun = &now
						}
						return false
					}

					if needDispatch {
						sv.taskwheel.Add(waitDur, fn)
					} else {
						fn()
						sv.jobManagerCache.lockedHandlers.Remove(jtask.Handler())
						opt.lastRun = &now
					}
				}
			}
		}()
	}
}

func (rw *GenericTypedHandler[T, U, E]) call_internal(r *Request, input T, fromcall bool) (output U, err error) {
	if rw.options == nil ||
		(!(fromcall && rw.options.Job.Async) &&
			!rw.options.Job.Dedupe &&
			!rw.options.Job.Cache &&
			r.memo.jobabortctrl == "") {

		return rw.handlefunc(r, input)
	}

	var zeroU U
	opt := rw.options
	asyncExec := fromcall && rw.options.Job.Async
	dedupeExec := rw.options.Job.Dedupe
	cacheExec := rw.options.Job.Cache

	var jec = jobExecutionContext{
		r:        r,
		opt:      rw.options,
		input:    input,
		fromcall: fromcall,
	}

	if err := jec.MarshalCheck(); err != nil {
		return zeroU, ErrJobInputEncodeFailed.With(err)
	}

	var syserr error
	unmarshalfn := func(meta JobInfo, outjson []byte, errjson []byte, serr error) (U, error, error) {
		var ji JobInfo
		var syserr error
		ji, output, err, syserr = unmarshalOutputSet[U, E](meta, outjson, errjson, serr)
		if syserr == nil {
			if opt.Job.CacheErrOnHit {
				return zeroU, ErrJobDuplicated, nil
			}

			if !hasMajorOrMinorVersionDiff(ji.Meta.Version, handlerVersion(opt)) {
				if !r.config.Log.Silent {
					r.logger.Info("job cache hit", zap.String("handler", jec.Handler()))
				}
				return output, err, nil
			}

			return zeroU, nil, FatalInvalidCacheError
		}

		return zeroU, nil, syserr
	}

	if cacheExec {
		output, err, syserr = unmarshalfn(opt.Job.callstrategy.Hit(r.Context(), jec.Handler(), jec.JobID(), jec.InputJSON()))
		if syserr == nil {
			return output, err
		}
	}

	var aquiredLock bool
	if asyncExec {
		var enqueued bool
		enqueued, err = opt.Job.callstrategy.Enqueue(
			r.Context(),
			jec.Handler(),
			jec.JobMeta("queued"),
			jec.JobID(),
			jec.InputJSON(),
			0)
		if err != nil {
			if !r.config.Log.Silent {
				r.logger.Info("job system error", zap.String("component", "requeue.enqueue"), zap.Error(err))
			}
			// db error etc.
			return zeroU, FatalBackendError.With(err)
		}

		if !r.config.Log.Silent {
			r.logger.Info("job queued", zap.String("handler", jec.Handler()))
		}
		if enqueued {
			return zeroU, NewJobPendingError(jec.JobID(), "job accepted")
		}
		return zeroU, NewJobPendingError(jec.JobID(), "job not finished yet")

	} else if dedupeExec {
		aquiredLock, err = opt.Job.callstrategy.Enqueue(
			r.Context(),
			jec.Handler(),
			jec.JobMeta("running"),
			jec.JobID(),
			jec.InputJSON(),
			0)
		if err != nil {
			if !r.config.Log.Silent {
				r.logger.Info("job system error", zap.String("component", "requeue.enqueue"), zap.Error(err))
			}
			// db error etc.
			return zeroU, FatalBackendError.With(err)
		}

		if !aquiredLock {
			if cacheExec {
				output, err, syserr = unmarshalfn(opt.Job.callstrategy.Hit(r.Context(), jec.Handler(), jec.JobID(), jec.InputJSON()))
				if syserr == nil {
					return output, err
				}

				// if job is not finished, wait it
				jnferr, ok := syserr.(*JobPendingError)
				if ok {
					output, err, syserr = unmarshalfn(opt.Job.callstrategy.Wait(r.Context(), jnferr.JobID, r.cache.taskwheel))
					if syserr == nil {
						return output, err
					}
				}

				if !r.config.Log.Silent {
					r.logger.Info("job lost", zap.String("handler", jec.Handler()), zap.Error(syserr))
				}
				return zeroU, FatalBackendError.With(syserr)
				// cache not found or expired
			}
			// if enqueued failed & dedupe, exec declined.
			if !r.config.Log.Silent {
				r.logger.Info("job duplicated", zap.String("handler", jec.Handler()))
			}
			return zeroU, ErrJobDuplicated
		}

		// Lock aquired
		leaset := r.config.JobConfig.LeaseDuration
		task := r.cache.taskwheel.Add(time.Duration(leaset/2), func() bool {
			err := opt.Job.callstrategy.LeaseUpdate(r.Context(), jec.JobID(), leaset)
			//err := s.LeaseUpdate(sv.appctx, jobn.key, leaset)
			if err != nil && !r.Config().Log.Silent {
				r.logger.Info("job system error", zap.String("component", "leaseupdate"), zap.Error(err))
			}
			return true
		})
		defer task.Cancel()
	}

	output, err = rw.handlefunc(r, input)
	if cacheExec {
		// only cache job is not aborted
		if r.memo.jobabortctrl == "" {
			outJSON, errJSON, syserr := marshalOutputSet[U, E](output, err)
			if syserr == nil {

				err := opt.Job.callstrategy.Done(
					r.Context(),
					jec.Handler(),
					jec.JobMeta("done"),
					jec.JobID(),
					jec.InputJSON(),
					outJSON,
					errJSON)
				if err != nil && !r.config.Log.Silent {
					r.logger.Info("job system error", zap.String("component", "done"), zap.Error(err))
				}
			}
		}
	} else if dedupeExec {
		err := opt.Job.callstrategy.Free(r.Context(), jec.JobID())
		if err != nil && !r.config.Log.Silent {
			r.logger.Info("job system error", zap.String("component", "free"), zap.Error(err))
		}
	}

	// mark requeue
	if r.memo.jobabortctrl == JOB_ABORT_REQUEUE || r.memo.jobabortctrl == JOB_ABORT_REQUEUE_AT {
		_, err = opt.Job.callstrategy.Enqueue(
			r.Context(),
			jec.Handler(),
			jec.JobMeta(jec.EnqueueStatus()),
			jec.JobID(),
			jec.InputJSON(),
			requeueDelay(r))
		if err != nil {
			if !r.config.Log.Silent {
				r.logger.Info("job system error", zap.String("component", "markrequeue"), zap.Error(err))
			}
		} else {
			if !r.config.Log.Silent {
				r.logger.Info("job requeued", zap.String("handler", jec.Handler()), zap.String("requestid", jec.JobID()))
			}
		}

	}

	return output, err
}

func (rw *GenericTypedHandler[T, U, E]) job_consume(r *Request, handler string, injson []byte) (key string, outputz []byte, errjsonz []byte, syserr error) {

	var innererr error
	input := NewRefOf[T](func(a any) {
		innererr = json.Unmarshal(injson, a)
	})
	if innererr != nil {
		return key, nil, nil, ErrJobInputDecodeFailed.With(innererr)
	}

	key = encodeJobID(handler, input, injson, "")

	// fill SelfDiscovery.
	if rw.options.hasSelfDiscovery {
		ferr := fillSelfDiscovery(input)
		if ferr != nil {
			return key, nil, nil, ErrJobInputDecodeFailed.With(ferr)
		}
	}

	outJSON, errJSON, syserr := marshalOutputSet[U, E](rw.handlefunc(r, input))
	return key, outJSON, errJSON, syserr
}

func (rw *GenericTypedHandler[T, U, E]) JobResult(r *Request, jobid string) (output U, err error) {
	var zeroU U

	jid, err := decodeJobID(jobid)
	if err != nil {
		return zeroU, err
	}

	if jid.Handler != encodeHandlerName(rw.options) {
		return zeroU, ErrJobHandlerMismatch
	}

	var ji JobInfo
	var syserr error
	ji, output, err, syserr = unmarshalOutputSet[U, E](rw.options.Job.callstrategy.Result(r.Context(), jobid))
	if syserr == nil {
		if !hasMajorOrMinorVersionDiff(ji.Meta.Version, handlerVersion(rw.options)) {
			return output, err
		}
		return zeroU, FatalInvalidCacheError
	} else {
		if !r.config.Log.Silent {
			r.logger.Info("job system error", zap.String("component", "jobresult"), zap.Error(syserr))
		}
	}

	return zeroU, syserr
}

type callStrategy interface {
	Init(ctx context.Context) error

	// Push job to queue / Aquire Lock
	Enqueue(ctx context.Context, handler string, meta *JobMeta, key string, injson []byte, delay_sec int) (enqueud bool, err error)

	// Pull queued job.
	Dequeue(ctx context.Context, handlers []string, lease_dur time.Duration, ema *EMACalculator) (jt JobTask, err error)

	// Search lease expired job and requeue.
	Reaping(ctx context.Context) (err error)

	// Free Lock
	Free(ctx context.Context, key string) (err error)

	//
	LeaseUpdate(ctx context.Context, key string, lease_dur time.Duration) (err error)

	// Func is called synchronously and Hit checks if its cached already.
	Hit(ctx context.Context, handler string, key string, injson []byte) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Function is called synchronously and then cache its result.
	Done(ctx context.Context, handler string, meta *JobMeta, key string, injson []byte, outjson []byte, errjson []byte) (err error)

	// Find completed job. (blocking)
	Wait(ctx context.Context, key string, tw *twWheel) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Find completed job. (non-blocking)
	Result(ctx context.Context, key string) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// List jobs
	List(ctx context.Context, statuses []string, offset, limit int) ([]JobInfo, error)
}

type JobTask interface {
	Key() string
	Handler() string
	Meta() *JobMeta
	Input() []byte

	Success(ctx context.Context, ttl *time.Time, outjson []byte, errjson []byte) (err error)
	Fail(ctx context.Context) (err error)
	HeartBeat(ctx context.Context, lease_dur time.Duration) (err error)
	Requeue(ctx context.Context, delay_sec int) error
}
