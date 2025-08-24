package allino

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

//type TypedRouter struct {
//	server            *Server
//	typedHandlerCache []TypedHandler
//	optionsCache      []*HandlerOption
//}

func (r *Server) TypedHandle(th TypedHandler) {
	opt := th.Options()
	if opt.CORS {
		r.Fiber.Add("OPTIONS", opt.Path, func(w *fiber.Ctx) error {
			addCORSHeaders(opt, w)
			w.Status(http.StatusOK)
			return nil
		})
	}

	requestFn := func(req *Request) {
		req.cache.options = th.Options()
		if req.fiber != nil {
			req.loggerWith = req.Logger().With(
				zap.String("method", req.fiber.Method()),
				zap.String("path", opt.Path),
				zap.String("ip", req.ClientIP()),
			)
		} else {
			req.loggerWith = req.Logger().With(
				zap.String("path", opt.Path),
				zap.String("ip", req.ClientIP()),
			)
		}

		th.HandleRequest(req)
	}

	r.typedHandlerCache = append(r.typedHandlerCache, th)
	r.HandleRequestFunc(opt.Method, opt.Path, requestFn)

	for _, m := range opt.SubMethod {
		r.HandleRequestFunc(m, opt.Path, requestFn)
	}
}

func (s *Server) TypedHandleWithPath(pattern string, th TypedHandler) {
	nth := th.Copy()
	nth.Options().Path = pattern
	s.TypedHandle(th)
}

func (r *Server) TypedHandleFiber(options HandlerOption, h fiber.Handler) {
	opt := &options
	if options.CORS {
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

func addCORSHeaders(options *HandlerOption, w *fiber.Ctx) {
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

func (r *Server) RegisteredTypedHandlers() []*HandlerOption {
	ho := make([]*HandlerOption, 0, 20)
	// 1. typedHandlerCache から
	for _, h := range r.typedHandlerCache {
		ho = append(ho, h.Options())
	}

	// 2. optionsCache から（通常の http.Handler も含める想定）
	ho = append(ho, r.optionsCache...)

	slices.SortFunc(ho, func(a, b *HandlerOption) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return ho
}
