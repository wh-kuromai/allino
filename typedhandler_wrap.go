package allino

type APIResponse[T any] struct {
	Data T `json:"data"`
}

type APIError[T error] struct {
	Err T `json:"error"`
}

func (e *APIError[T]) Error() string {
	return e.Err.Error()
}
