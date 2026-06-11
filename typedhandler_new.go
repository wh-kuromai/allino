package allino

import (
	"html/template"
	"reflect"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
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
	Package string
	Extra   any

	// Session
	Session SessionOption

	// Job
	Name    string
	Version string
	Cron    string
	JobMode string
	Job     JobOption

	// Logs
	AutoAudit    bool
	AutoAuditMsg string

	// Semantics
	Internal    bool
	Summary     string
	Description string
	Class       string

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

	inputType        reflect.Type
	outputType       reflect.Type
	errorType        reflect.Type
	eiserror         bool
	hasSelfDiscovery bool
	inputReflectPlan *reflectPlan

	lastRun *time.Time
	exts    *sync.Map
	cronid  cron.EntryID
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
		setDefault(v)
	} else {
		setDefault(&v)
	}

	h.exts.Store(t, handlerExtEntry{v, true})
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

func GetClassHandlers(class string) []TypedHandler {
	list := make([]TypedHandler, 0, len(typedHandlerList))
	for _, th := range typedHandlerList {
		if th.Options().Class == class {
			list = append(list, th)
		}
	}
	return list
}
