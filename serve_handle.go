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
	HandleRequest(r *Request)
}

type RequestHandlerFunc func(r *Request)

const (
	HTML = "text/html"
	JSON = "application/json"
)

func (s *Server) HandleRequestFunc(method, pattern string, handlerfunc func(*Request)) {
	s.Fiber.Add(method, pattern, func(w *fiber.Ctx) error {
		req := NewRequest(s, w)
		handlerfunc(req)
		return nil
	})
}

//func (s *Server) TypedHandle(th TypedHandler) {
//	s.TypedRouter.TypedHandle(th)
//}

//func (s *Server) TypedHandleFiber(options HandlerOption, h fiber.Handler) {
//	s.TypedRouter.TypedHandleFiber(options, h)
//}
