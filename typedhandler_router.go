package allino

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/wh-kuromai/allino/internal/randcron"
)

//type TypedRouter struct {
//	server            *Server
//	FunctionCache []Function
//	optionsCache      []*Option
//}

func (r *Server) TypedHandle(th Function) {
	if th.Options().Cron != "" {

		rcron, err := randcron.Expand(th.Options().Cron, r.ServerID())
		if err != nil {
			r.Logger.Error("cron error", zap.String("spec", rcron), zap.String("handler", th.Options().Name))
		} else {
			eid, err := r.Cron.AddFunc(rcron, func() {
				rr := NewRuntime(r, nil)
				defer rr.do_defer()
				rr.cache.req_type = REQUEST_CRON
				th.Handlefunc(rr, nil)
			})
			if err == nil {
				th.Options().cronid = eid
			} else {
				r.Logger.Error("cron error", zap.String("spec", rcron), zap.String("handler", th.Options().Name))
			}
		}
	}

	opt := th.Options()
	if opt.Path == "" {
		r.internalHandlerCache = append(r.internalHandlerCache, th)
		return
	}

	if opt.CORS || (r.Config != nil && r.Config.Debug) {
		r.Fiber.Add("OPTIONS", opt.Path, func(w *fiber.Ctx) error {
			addCORSHeaders(opt, w)
			w.Status(http.StatusOK)
			return nil
		})
	}

	requestFn := func(req *Runtime) {
		//req.cache.options = th.Options()
		if req.fiber != nil {
			req.loggerWith = req.Logger().With(
				zap.String("method", req.fiber.Method()),
				zap.String("path", opt.Path),
				zap.String("ip", req.ClientIP()),
			)

			if opt.CORS || r.Config.Debug {
				addCORSHeaders(opt, req.Fiber())
			}
		} else {
			req.loggerWith = req.Logger().With(
				zap.String("path", opt.Path),
				zap.String("ip", req.ClientIP()),
			)
		}

		th.HandleRequest(req)
	}

	r.FunctionCache = append(r.FunctionCache, th)
	r.HandleRequestFunc(opt.Method, opt.Path, requestFn)

	for _, m := range opt.SubMethod {
		r.HandleRequestFunc(m, opt.Path, requestFn)
	}
}

func (s *Server) TypedHandleWithPath(pattern string, th Function) {
	nth := th.Copy()
	nth.Options().Path = pattern
	s.TypedHandle(th)
}

func (r *Server) TypedHandleFiber(options Option, h fiber.Handler) {
	opt := &options
	if options.CORS || r.Config.Debug {
		r.Fiber.Add("OPTIONS", options.Path, func(w *fiber.Ctx) error {
			addCORSHeaders(opt, w)
			w.Status(http.StatusOK)
			return nil
		})
	}
	r.optionsCache = append(r.optionsCache, opt)
	r.Fiber.Add(opt.Method, opt.Path, h)

	for _, m := range opt.SubMethod {
		r.Fiber.Add(m, opt.Path, h)
	}
}

func addCORSHeaders(options *Option, w *fiber.Ctx) {
	if options != nil && options.CORSCustomHeader != nil {
		for k, v := range options.CORSCustomHeader {
			w.Set(k, v)
		}
		return
	}

	w.Set("Access-Control-Allow-Origin", "*")
	w.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func (r *Server) RegisteredFunctions() []*Option {
	ho := make([]*Option, 0, 20)
	// 1. FunctionCache から
	for _, h := range r.FunctionCache {
		ho = append(ho, h.Options())
	}

	// 2. optionsCache から（通常の http.Handler も含める想定）
	ho = append(ho, r.optionsCache...)

	slices.SortFunc(ho, func(a, b *Option) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return ho
}

func (r *Server) RegisteredInternalFunctionHandlers() []Function {
	handlers := make([]Function, len(r.internalHandlerCache))
	copy(handlers, r.internalHandlerCache)
	return handlers
}

func (r *Server) RegisteredFiberOptions() []*Option {
	options := make([]*Option, len(r.optionsCache))
	copy(options, r.optionsCache)
	return options
}
