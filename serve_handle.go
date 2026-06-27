package allino

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func (s *Server) HandleFunc(method, pattern string, handlefunc http.HandlerFunc) {
	s.HandleFiber(method, pattern, func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(handlefunc)(c.Context())
		return nil
	})
}
func (s *Server) HandleFiber(method, pattern string, handlefunc fiber.Handler) {
	s.Fiber.Add(method, pattern, handlefunc)
}

type RequestHandler interface {
	HandleRequest(r *Runtime)
}

type RequestHandlerFunc func(r *Runtime)

const (
	HTML = "text/html"
	JSON = "application/json"
)

func (s *Server) HandleRequestFunc(method, pattern string, handlerfunc func(*Runtime)) {
	s.Fiber.Add(method, pattern, func(w *fiber.Ctx) error {
		req := NewRequest(s, w)
		defer req.do_defer()
		req.cache.req_type = REQUEST_HTTP
		handlerfunc(req)
		return nil
	})
}

//func (s *Server) TypedHandle(th Function) {
//	s.TypedRouter.TypedHandle(th)
//}

//func (s *Server) TypedHandleFiber(options Option, h fiber.Handler) {
//	s.TypedRouter.TypedHandleFiber(options, h)
//}
