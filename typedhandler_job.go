package allino

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
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

	OnCacheUpgrade func(version string, updated_at time.Time, old_output []byte) (TransformAction, any, error) `json:"-"`

	//Backend string

	callstrategy callStrategy `json:"-"`
}

type TransformAction int

const (
	TransformActionNone TransformAction = iota
	TransformActionUse
	TransformActionFallback
	TransformActionFail
)

var JobExtension = NewExtension[any, any](
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
				if s.jobManager == nil {
					s.jobManager = newJobManager()
				}

				if callSQLstrategy == nil {
					callSQLstrategy = newcallSQLStrategy(s)
					err := callSQLstrategy.Init(virtual.Context(), (s.Config.SQL.AllowMigrate != nil && *s.Config.SQL.AllowMigrate))
					if err != nil && !s.Config.Log.Silent {
						s.Logger.Error("job system error", zap.String("component", "register"), zap.Error(err))
					}
					s.jobManager.WorkerInit(callSQLstrategy, s)

					// Reaping
					s.TimeWheel.Add(s.Config.JobConfig.LeaseDuration, func() bool {
						err = callSQLstrategy.Reaping(s.appctx)
						if err != nil && !s.Config.Log.Silent {
							s.Logger.Error("job system error", zap.String("component", "reaping"), zap.Error(err))
						}
						return true
					})
				}

				opt.Job.callstrategy = callSQLstrategy
				s.jobManager.handlers.Add(encodeHandlerName(opt))
			}

			s.handlerOptMap[encodeHandlerName(opt)] = opt
			return nil
		},

		OnShutdown: func(s *Server, virtual *Request) error {
			return nil
		},
	},
)

var ErrHandlerNotFound = NewError("handler not found")

func find_handler(sv *Server, handler string) (string, error) {
	_, ok := sv.handlerOptMap[handler]
	if ok {
		return handler, nil
	}

	for k, _ := range sv.handlerOptMap {
		idx := strings.Index(k, "@")
		if idx > 0 && k[:idx] == handler {
			return k, nil
		}
	}

	return "", ErrHandlerNotFound
}

// called from CLI
func call_direct(sv *Server, r *Request, handler string, injson []byte, infunc func(input any) error) (key string, outjson []byte, err []byte, syserr error) {

	opt := sv.handlerOptMap[handler]

	var jec = jobExecutionContext{
		r:               r,
		opt:             opt,
		inJSON:          injson,
		inJSONMarshaled: true,
		fromcall:        true,
	}

	r.cache.req_type = REQUEST_CLI
	r.cache.parentjobid = jec.JobID()
	r.cache.rootjobid = jec.JobID()

	return opt.consumer(r, encodeHandlerName(opt), injson, true, infunc)
}

func unmarshalfnMake[U any, E error](r *Request, opt *HandlerOption, upool *ReflectPool[U], epool *ReflectPool[E], handler string) func(ji JobInfo, outjson []byte, errjson []byte, serr error) (output U, err error, syserr error) {
	return func(ji JobInfo, outjson []byte, errjson []byte, serr error) (output U, err error, syserr error) {

		var zeroU U
		now := time.Now()

		// cache expired or version mismatch
		if hasMajorOrMinorVersionDiff(ji.Meta.Version, handlerVersion(opt)) ||
			(ji.Meta.TTL != nil && now.After(*ji.Meta.TTL)) {

			// try cache transform
			if opt.Job.OnCacheUpgrade != nil && outjson != nil && errjson == nil {

				result, newout, newerr := opt.Job.OnCacheUpgrade(ji.Meta.Version, ji.UpdatedAt, outjson)
				switch result {
				case TransformActionUse:
					newoutput, ok := newout.(U)
					if ok {
						ji.Meta.Version = handlerVersion(opt)
						ji.UpdatedAt = now
						newoutjson, _, newsyserr := marshalOutputSet[U, E](newoutput, nil)
						if newsyserr != nil {
							if !r.config.Log.Silent {
								r.logger.Info("cache transform failed: marshal error", zap.String("handler", handler), zap.Error(newsyserr))
							}
							return zeroU, FatalTransformFailed, nil
						}

						// update cache and return new output
						syserr = opt.Job.callstrategy.Done(r.Context(), encodeHandlerName(opt), &ji.Meta, ji.JobID, nil, newoutjson, nil)
						if syserr != nil {
							if !r.config.Log.Silent {
								r.logger.Info("cache transform failed: done error", zap.String("handler", handler), zap.Error(newsyserr))
							}
							return zeroU, FatalTransformFailed, nil
						}
						return newoutput, nil, nil
					}
					if !r.config.Log.Silent {
						r.logger.Info("cache transform failed: output type not match", zap.String("handler", handler))
					}
					return zeroU, FatalTransformFailed, nil

				case TransformActionFallback:
					// return syserr -> cache hit failed -> fallback to execute.
					return zeroU, nil, ErrJobNotFound
				case TransformActionFail:
					// respond newerr
					return zeroU, newerr, nil
				}

				return zeroU, FatalTransformFailed, nil
			}

			// fallback if no transformer set.
			return zeroU, nil, FatalInvalidCacheError
		}

		_, output, err, syserr = unmarshalOutputSet[U, E](ji, upool, epool, outjson, errjson, serr)
		if syserr != nil {
			return zeroU, nil, syserr
		}

		if opt.Job.CacheErrOnHit {
			return zeroU, ErrJobDuplicated, nil
		}

		if !r.config.Log.Silent {
			r.logger.Info("job cache hit", zap.String("handler", handler))
		}
		return output, err, nil
	}
}

// called from HTTP or Handler.Call
func (rw *GenericTypedHandler[T, U, E]) call_job(r *Request, input T, fromcall bool) (output U, err error) {

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

	r.cache.req_type = REQUEST_CLI
	r.cache.parentjobid = jec.JobID()
	r.cache.rootjobid = jec.JobID()

	var syserr error

	unmarshalfn := unmarshalfnMake[U, E](r, opt, rw.upool, rw.epool, jec.Handler())
	if cacheExec {
		output, err, syserr = unmarshalfn(opt.Job.callstrategy.Hit(r.Context(), jec.Handler(), opt.Job.CacheExpire != 0, jec.JobID(), jec.InputJSON()))

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
			jec.JobMeta(statusQueued),
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

		if enqueued {
			if !r.config.Log.Silent {
				r.logger.Info("job queued", zap.String("handler", jec.Handler()))
			}
			return zeroU, NewJobPendingError(jec.JobID(), "job accepted")
		}
		return zeroU, NewJobPendingError(jec.JobID(), "job not finished yet")

	} else if dedupeExec {
		aquiredLock, err = opt.Job.callstrategy.Enqueue(
			r.Context(),
			jec.Handler(),
			jec.JobMeta(statusLeased),
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
				output, err, syserr = unmarshalfn(opt.Job.callstrategy.Hit(r.Context(), jec.Handler(), opt.Job.CacheExpire != 0, jec.JobID(), jec.InputJSON()))
				if syserr == nil {
					return output, err
				}

				// if job is not finished, wait it
				jnferr, ok := syserr.(*JobPendingError)
				if ok {
					output, err, syserr = unmarshalfn(opt.Job.callstrategy.Wait(r.Context(), jnferr.JobID, rw.options.Job.CacheExpire != 0, r.server.TimeWheel))
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
		task := r.server.TimeWheel.Add(time.Duration(leaset/2), func() bool {
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
					jec.JobMeta(statusDone),
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

// called from CLI or Worker
func (rw *GenericTypedHandler[T, U, E]) job_consume(r *Request, handler string, injson []byte, direct bool, infunc func(input any) error) (key string, outputz []byte, errjsonz []byte, syserr error) {

	input, innererr := rw.tpool.New(func(a any) error {
		return json.Unmarshal(injson, a)
	})

	//var innererr error
	//input := NewRefOf[T](func(a any) {
	//	innererr = json.Unmarshal(injson, a)
	//})
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

	if direct {
		var t *T
		if reflect.TypeOf(t).Elem().Kind() == reflect.Ptr {
			err := r.getAll(input, rw.options.inputReflectPlan)
			if err != nil {
				return "", nil, nil, err
			}
		} else {
			err := r.getAll(&input, rw.options.inputReflectPlan)
			if err != nil {
				return "", nil, nil, err
			}
		}
	}

	if infunc != nil {
		err2 := infunc(input)
		if err2 != nil {
			return "", nil, nil, err2
		}
	}

	outJSON, errJSON, syserr := marshalOutputSet[U, E](rw.handlefunc(r, input))
	return key, outJSON, errJSON, syserr
}

func (rw *GenericTypedHandler[T, U, E]) JobResult(r *Request, jobid string) (output U, err error) {
	var zeroU U
	var syserr error

	jid, err := decodeJobID(jobid)
	if err != nil {
		return zeroU, err
	}

	if jid.Handler != encodeHandlerName(rw.options) {
		return zeroU, ErrJobHandlerMismatch
	}

	unmarshalfn := unmarshalfnMake[U, E](r, rw.options, rw.upool, rw.epool, jid.Handler)
	output, err, syserr = unmarshalfn(rw.options.Job.callstrategy.Result(r.Context(), jobid, rw.options.Job.CacheExpire != 0))

	if syserr == nil {
		return output, err
	}

	if !r.config.Log.Silent {
		r.logger.Info("job system error", zap.String("component", "jobresult"), zap.Error(syserr))
	}

	/*
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
	*/
	return zeroU, syserr
}

type callStrategy interface {
	Init(ctx context.Context, allow_migrate bool) error

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
	Hit(ctx context.Context, handler string, volatile bool, key string, injson []byte) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Function is called synchronously and then cache its result.
	Done(ctx context.Context, handler string, meta *JobMeta, key string, injson []byte, outjson []byte, errjson []byte) (err error)

	// Find completed job. (blocking)
	Wait(ctx context.Context, key string, volatile bool, tw *TimeWheel) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// Find completed job. (non-blocking)
	Result(ctx context.Context, key string, volatile bool) (meta JobInfo, outjson []byte, errjson []byte, err error)

	// List jobs
	List(ctx context.Context, statuses []int, offset, limit int) ([]JobInfo, error)

	// Count jobs
	Total(ctx context.Context, jobid ...string) (map[string]int, error)
}

type JobTask interface {
	Key() string
	Handler() string
	Meta() *JobMeta
	Input() []byte

	Success(ctx context.Context, handler string, meta *JobMeta, key string, injson []byte, outjson []byte, errjson []byte) (err error)
	Fail(ctx context.Context) (err error)
	HeartBeat(ctx context.Context, lease_dur time.Duration) (err error)
	Requeue(ctx context.Context, delay_sec int) error
}
