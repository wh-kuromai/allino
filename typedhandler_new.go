package allino

import (
	"html/template"
	"reflect"
	"sync"
	"time"
)

type HandlerOption struct {
	// HTTP
	Path               string
	Method             string
	SubMethod          []string
	ContentType        string
	CORS               bool
	CORSCustomHeader   map[string]string
	RequestHandler     func(r *Request, input any) (consumed bool, err error)
	ResponseHandler    func(r *Request, output any) (consumed bool)
	ResponseStatusCode int
	ErrorHandler       func(r *Request, err error) (consumed bool)
	ErrorStatusCode    int
	RedirectStatusCode int
	NoWrapJSON         bool
	HTMLTemplate       string

	// Custom field
	Extra any

	// Job
	Name    string
	Version string
	JobMode string
	Job     JobOption

	// Logs
	AutoAudit    bool
	AutoAuditMsg string

	// Semantics
	Internal    bool
	Summary     string
	Description string

	// Reflection Hints
	InputTypeHint  any
	OutputTypeHint any
	ErrorTypeHint  any

	// Events
	OnInit     func(s *Server, r *Request) error
	OnShutdown func(s *Server, r *Request) error

	// cache
	parsedTemplate *template.Template
	consumer       jobconsumer

	inputType  reflect.Type
	outputType reflect.Type
	errorType  reflect.Type
	eiserror   bool

	lastRun *time.Time
	exts    *sync.Map
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
