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

type Error struct {
	Msg string `json:"msg"`
}

func (e Error) Error() string {
	return e.Msg
}
