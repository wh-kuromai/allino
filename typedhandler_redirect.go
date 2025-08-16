package allino

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type FiberHandler interface {
	HandleFiber(w *fiber.Ctx)
}

type RedirectError struct {
	StatusCode int
	Location   string
	Err        error
}

func (e *RedirectError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.StatusCode) + " redirect to " + e.Location
}

func (e *RedirectError) HandleFiber(w *fiber.Ctx) {
	w.Set("Location", e.Location)
	w.Status(e.StatusCode)
}

/*
func (e *RedirectError) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Location", e.Location)
	w.WriteHeader(e.StatusCode)
}
*/

func NewRedirectError(status int, location string) *RedirectError {
	return &RedirectError{
		StatusCode: status,
		Location:   location,
	}
}
