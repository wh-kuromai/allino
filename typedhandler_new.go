package allino

import (
	"html/template"
	"reflect"
	"sync"
)

type HandlerOption struct {
	Path               string
	Priority           int
	Method             string
	SubMethod          []string
	ContentType        string
	CORS               bool
	CORSCustomHeader   map[string]string
	RequestHandler     func(r *Request, input any) error
	ResponseHandler    func(r *Request, output any)
	ResponseStatusCode int
	ErrorHandler       func(r *Request, err error)
	ErrorStatusCode    int
	RedirectStatusCode int
	NoWrapJSON         bool
	HTMLTemplate       string
	AutoAudit          bool
	AutoAuditMsg       string

	parsedTemplate *template.Template

	OnInit     func(s *Server) error
	OnShutdown func(s *Server) error

	Summary        string
	Description    string
	InputTypeHint  any
	OutputTypeHint any
	ErrorTypeHint  any

	inputType  reflect.Type
	outputType reflect.Type
	errorType  reflect.Type

	exts *sync.Map
}

func (h HandlerOption) InputType() reflect.Type {
	return h.inputType
}

func (h HandlerOption) OutputType() reflect.Type {
	return h.outputType
}

func (h HandlerOption) ErrorType() reflect.Type {
	return h.errorType
}

type handlerExtEntry struct {
	value     any  // 実際の *F
	isUserSet bool // Handler 作成時にユーザーが明示設定したか
}

// 追加オプション
func (h HandlerOption) WithExt(v any) HandlerOption {
	if h.exts == nil {
		h.exts = &sync.Map{}
	}
	t := reflect.ValueOf(v).Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		setDefault(v)
	} else {
		setDefault(&v)
	}

	// --- copy-on-write ---
	//m2 := make(map[reflect.Type]any, len(h.exts)+1)
	//for k, vv := range h.exts {
	//	m2[k] = vv
	//}
	//m2[t] = v
	if t.Kind() == reflect.Ptr {
		h.exts.Store(t, handlerExtEntry{v, true})
	} else {
		h.exts.Store(t, handlerExtEntry{&v, true})
	}
	return h
}

func NewTypedAPI[T, U any, E error](path string, handler func(*Request, T) (U, E)) *GenericTypedHandler[T, U, E] {
	return NewTypedHandler(
		HandlerOption{
			Path:        path,
			Method:      "GET",
			SubMethod:   []string{"POST"},
			ContentType: JSON,
		},
		handler,
	)
}

func NewTypedUI[T, U any, E error](path string, handler func(*Request, T) (U, E)) *GenericTypedHandler[T, U, E] {
	return NewTypedHandler(
		HandlerOption{
			Path:        path,
			Method:      "GET",
			SubMethod:   []string{"POST"},
			ContentType: HTML,
		},
		handler,
	)
}
