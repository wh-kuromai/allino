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
				//}
				err := opt.Job.callstrategy.Register(virtual.Context(), encodeHandlerName(opt), opt)
				if err != nil && !s.Config.Log.Silent {
					s.Logger.Info("job system error", zap.String("component", "register"), zap.Error(err))
				}
			}

			return nil
		},

		OnShutdown: func(s *Server, virtual *Request) error {
			return nil
		},
	},
)

type jobTask struct {
	key      string
	backend  string
	handler  string
	injson   []byte
	parentid string
	rootid   string
}

func strategyWorkerInit(s *callSQLStrategy, sv *Server) {
	idleit := sv.Config.JobConfig.IdleInterval
	//leasesec := int(mergeSingle(sv.Config.JobConfig.LeaseSeconds, opt.Job.LeaseSeconds).Seconds())

	leaset := sv.Config.JobConfig.LeaseDuration

	jobchan := make(chan jobTask, sv.Config.JobConfig.Concurrency*2)

	go func() {
		for {
			select {
			case <-sv.appctx.Done():
				return
			default:
				hs := s.handlers.Diff(s.lockedHandlers)
				key, handler, meta, injson, err := s.Dequeue(sv.appctx, hs, leaset)
				if err != nil {
					if !errors.Is(err, ErrJobNotFound) {
						sv.Logger.Error("job system error", zap.String("component", "dequeue"), zap.Error(err))
					}
					time.Sleep(idleit)
					continue
				}

				jt := jobTask{
					key:      key,
					backend:  s.name,
					handler:  handler,
					injson:   injson,
					parentid: key,
					rootid:   meta.RootID,
				}

				if jt.rootid == "" {
					jt.rootid = key
				}

				// if dequeued need handler lock,
				opt := s.handlerOptMap[jt.handler]
				if opt.Job.Interval != 0 {
					s.lockedHandlers.Add(jt.handler)
				}

				jobchan <- jt
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
				case jobn := <-jobchan:
					now := time.Now()
					opt := s.handlerOptMap[jobn.handler]

					// check if need to re-dispatch
					needDispatch := false
					var waitDur time.Duration
					if opt.Job.Interval != 0 && opt.lastRun != nil {
						waitDur = time.Until((*opt.lastRun).Add(time.Duration(float64(opt.Job.Interval) * s.dequeueThroughputEMA.CurrentAverage)))
						if waitDur > 0 {
							needDispatch = true
						}
					}

					task := sv.taskwheel.Add(time.Duration(leaset/2), func() bool {
						err := s.LeaseUpdate(sv.appctx, jobn.key, leaset)
						if err != nil && !sv.Config.Log.Silent {
							sv.Logger.Info("job system error", zap.String("component", "leaseupdate"), zap.Error(err))
						}
						return true
					})

					fn := func() bool {
						r := NewRequest(sv, nil)
						r.cache.req_type = REQUEST_JOB
						r.cache.requestid = jobn.key
						r.cache.parentjobid = jobn.parentid
						r.cache.rootjobid = jobn.rootid

						if !r.config.Log.Silent {
							r.logger.Info("job started", zap.String("handler", jobn.handler), zap.String("requestid", jobn.key))
						}
						_, outjson, errjson, syserr := opt.consumer(r, jobn.handler, jobn.injson)
						if syserr != nil {
							if !r.config.Log.Silent {
								r.logger.Info("job failed", zap.String("handler", jobn.handler), zap.String("requestid", jobn.key), zap.Error(syserr))
							}
							err := s.Free(sv.appctx, jobn.key)
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/syserr"), zap.Error(err))
							}
						} else if r.memo.jobabortctrl != "" {
							// cancel if abort or error
							if !r.config.Log.Silent {
								r.logger.Info("job requeued", zap.String("handler", jobn.handler), zap.String("requestid", jobn.key))
							}
							err := s.Requeue(sv.appctx, jobn.key, requeueDelay(r))
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
								r.logger.Info("job completed (cache)", zap.String("handler", jobn.handler), zap.String("requestid", jobn.key))
							}

							err := s.DoneAsync(sv.appctx, jobn.key, ttl, outjson, errjson)
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/doneasync"), zap.Error(err))
							}
						} else {
							if !r.config.Log.Silent {
								r.logger.Info("job completed", zap.String("handler", jobn.handler), zap.String("requestid", jobn.key))
							}
							err := s.Free(sv.appctx, jobn.key)
							if err != nil && !r.config.Log.Silent {
								r.logger.Info("job system error", zap.String("component", "dequeue/etc"), zap.Error(err))
							}
						}
						task.Cancel()
						if needDispatch {
							s.lockedHandlers.Remove(jobn.handler)
							opt.lastRun = &now
						}
						return false
					}

					if needDispatch {
						sv.taskwheel.Add(waitDur, fn)
					} else {
						fn()
						s.lockedHandlers.Remove(jobn.handler)
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

	var enqueued bool
	if asyncExec || dedupeExec {
		enqueued, err = opt.Job.callstrategy.Enqueue(
			r.Context(),
			jec.Handler(),
			jec.JobMeta(jec.EnqueueStatus()),
			jec.JobID(),
			jec.InputJSON(),
			0)
		if err != nil {
			if !r.config.Log.Silent {
				r.logger.Info("job system error", zap.String("component", "requeue.enqueue"), zap.Error(err))
			}
			// db error etc.
			return zeroU, FatalBackendError.With(err)
		} else if enqueued {

			// if enqueued & async, return JobAcceptedError
			if asyncExec {
				if !r.config.Log.Silent {
					r.logger.Info("job queued", zap.String("handler", jec.Handler()))
				}
				jerr := &JobAcceptedError{
					Status: 202,
					JobID:  jec.JobID(),
					Msg:    "job successfully accepted",
				}

				return zeroU, jerr
			}

			// do nothing if enqueued & dedupe, exec approved
		} else {
			// do nothing if enqueued failed & async, proceed cache check

			// if enqueued failed & dedupe, exec declined.
			if dedupeExec {
				if !r.config.Log.Silent {
					r.logger.Info("job duplicated", zap.String("handler", jec.Handler()))
				}
				return zeroU, ErrJobDuplicated
			}

		}
	}

	// cache check
	if cacheExec {
		var ji JobInfo
		var syserr error

		ji, output, err, syserr = unmarshalOutputSet[U, E](opt.Job.callstrategy.Hit(r.Context(), jec.Handler(), jec.JobID(), jec.InputJSON()))
		if syserr == nil {
			if opt.Job.CacheErrOnHit {
				return zeroU, ErrJobDuplicated
			}

			if !hasMajorOrMinorVersionDiff(ji.Meta.Version, handlerVersion(opt)) {
				if !r.config.Log.Silent {
					r.logger.Info("job cache hit", zap.String("handler", jec.Handler()))
				}
				return output, err
			}

			// cache found but cached version is not what we expected.

			return zeroU, FatalInvalidCacheError
		}

		// if job is not finished, wait it
		jnferr, ok := syserr.(*JobNotFinishedError)
		if ok {
			ji, output, err, syserr = unmarshalOutputSet[U, E](opt.Job.callstrategy.Wait(r.Context(), jnferr.JobID, r.cache.taskwheel))
			if syserr == nil {
				if !hasMajorOrMinorVersionDiff(ji.Meta.Version, handlerVersion(opt)) {
					return output, err
				}
				// cache found but cached version is not what we expected.
				return zeroU, FatalInvalidCacheError
			}
		}

		// cache not found or expired
	}

	// return error if async job is already enqueued but not finished yet.
	if asyncExec {
		return zeroU, NewJobNotFinishedError(jec.JobID())
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

		enqueued, err = opt.Job.callstrategy.Enqueue(
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
	ferr := fillSelfDiscovery(input)
	if ferr != nil {
		return key, nil, nil, ErrJobInputDecodeFailed.With(ferr)
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

	Register(ctx context.Context, handler string, opt *HandlerOption) error

	// Find completed job. (blocking)
	Wait(ctx context.Context, key string, tw *twWheel) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Find completed job. (non-blocking)
	Result(ctx context.Context, key string) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Push job to queue.
	Enqueue(ctx context.Context, handler string, meta JobMeta, key string, injson []byte, delay_sec int) (enqueud bool, err error)

	// Pull queued job.
	Dequeue(ctx context.Context, handlers []string, lease_dur time.Duration) (key string, handler string, meta JobMeta, injson []byte, err error)

	// Search lease expired job and requeue.
	Reaping(ctx context.Context) (err error)

	// Update lease time for dequeued job.
	LeaseUpdate(ctx context.Context, key string, lease_dur time.Duration) (err error)

	// Function is called asynchronously and then store its result.
	DoneAsync(ctx context.Context, key string, ttl *time.Time, outjson []byte, errjson []byte) (err error)

	// Delete queue.
	Free(ctx context.Context, key string) (err error)

	// Func is called synchronously and Hit checks if its cached already.
	Hit(ctx context.Context, handler string, key string, injson []byte) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Function is called synchronously and then cache its result.
	Done(ctx context.Context, handler string, meta JobMeta, key string, injson []byte, outjson []byte, errjson []byte) (err error)

	// List jobs
	List(ctx context.Context, statuses []string, offset, limit int) ([]JobInfo, error)

	Requeue(ctx context.Context, key string, delay_sec int) error
}
