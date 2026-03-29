package allino

import (
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"sort"
	"strings"

	"go.uber.org/zap"
)

type TypedHandler interface {
	Options() *HandlerOption
	Copy() TypedHandler
	HandleRequest(r *Request)
	HandleCall(r *Request, input any) (output any, err error)
}

var typedHandlerList []TypedHandler

type idxhandler struct {
	i int
	h TypedHandler
}

// パスの優先度を判定するロジック
func comparePaths(p1, p2 string) bool {
	s1 := strings.Split(strings.Trim(p1, "/"), "/")
	s2 := strings.Split(strings.Trim(p2, "/"), "/")

	for k := 0; k < len(s1) && k < len(s2); k++ {
		char1 := s1[k]
		char2 := s2[k]

		// スコア付け (低いほど優先度が高い)
		// 静的: 0, パラメータ(:): 1, ワイルドカード(*): 2
		score1 := getPriority(char1)
		score2 := getPriority(char2)

		if score1 != score2 {
			return score1 < score2
		}
	}

	// セグメントが同じ場合は、より長いパス（深い階層）を先に持ってくる
	return len(s1) > len(s2)
}

func getPriority(segment string) int {
	if strings.HasPrefix(segment, ":") {
		return 1
	}
	if strings.Contains(segment, "*") {
		return 2
	}
	return 0 // 静的パス
}

func (s *Server) RegisterAllTypedHandler() {
	list := make([]*idxhandler, len(typedHandlerList))
	for i, h := range typedHandlerList {
		list[i] = &idxhandler{i, h}
	}

	//slices.SortFunc(list, func(a, b *idxhandler) int {
	//	r := cmp.Compare(a.h.Options().Priority, b.h.Options().Priority)
	//	if r == 0 {
	//		cmp.Compare(a.i, b.i)
	//	}
	//	return r
	//})

	sort.Slice(list, func(i, j int) bool {
		return comparePaths(list[i].h.Options().Path, list[j].h.Options().Path)
	})

	for _, l := range list {
		s.TypedHandle(l.h)
	}
}

func NewTypedHandler[T, U any, E error](option HandlerOption, handlefunc func(r *Request, input T) (output U, err E)) *GenericTypedHandler[T, U, E] {
	options := &option
	if options.Method == "" {
		options.Method = "GET"
	}

	if options.ResponseStatusCode == 0 {
		options.ResponseStatusCode = 200
	}
	if options.ErrorStatusCode == 0 {
		options.ErrorStatusCode = 400
	}
	if options.RedirectStatusCode == 0 {
		options.RedirectStatusCode = 302
	}

	var tDefault T
	tDefault = NewRefOf[T](func(a any) {
		setDefault(a)
	})

	/*
		var tDefault T
		tType := reflect.TypeOf(tDefault)
		if tType != nil {
			if reflect.ValueOf(tDefault).Kind() == reflect.Ptr {
				tDefault = reflect.New(tType.Elem()).Interface().(T)
				setDefault(tDefault)
				//defaults.Set(tDefault)
			} else {
				tDefaultPtr := reflect.New(tType).Interface()
				setDefault(tDefaultPtr)
				//defaults.Set(&tDefault)
				tDefault = reflect.ValueOf(tDefaultPtr).Elem().Interface().(T)
			}
		}
	*/

	var t *T
	var u *U
	var k *E
	var newParamFn func() T
	var inputReflectPlan *reflectPlan

	if options.NoWrapJSON {
		options.outputType = reflect.TypeOf(u).Elem()
		options.errorType = reflect.TypeOf(k).Elem()
	} else {
		var uw *APIResponse[U]
		var ew *APIError[E]
		options.outputType = reflect.TypeOf(uw).Elem()
		options.errorType = reflect.TypeOf(ew).Elem()
	}
	options.inputType = reflect.TypeOf(t).Elem()
	inputReflectPlan = buildPlan(option.inputType)

	if reflect.TypeOf(k).Elem() == reflect.TypeOf((*error)(nil)).Elem() {
		options.eiserror = true
	}

	if options.InputTypeHint != nil {
		options.inputType = reflect.TypeOf(options.InputTypeHint)
	}

	if options.OutputTypeHint != nil {
		options.outputType = reflect.TypeOf(options.OutputTypeHint)
	}

	if options.ErrorTypeHint != nil {
		options.errorType = reflect.TypeOf(options.ErrorTypeHint)
	}

	if options.inputType.Kind() == reflect.Ptr {
		t := options.inputType.Elem()
		newParamFn = func() T {
			cp := reflect.New(t)
			cp.Elem().Set(reflect.ValueOf(tDefault).Elem())
			return cp.Interface().(T)
		}
	}

	var contentTypeHandlerThis = contentTypeHandlerMap[options.ContentType]
	//var contentTypeHandlerJSON = contentTypeHandlerMap[JSON]
	var contentTypeHandlerHTML = contentTypeHandlerMap[HTML]

	var rw *GenericTypedHandler[T, U, E]
	rw = &GenericTypedHandler[T, U, E]{
		options:    options,
		handlefunc: handlefunc,
		handler: func(r *Request) {
			if options.ContentType != "" {
				r.fiber.Set("Content-Type", options.ContentType)
			}

			var param T
			var err error
			// instantiate param if it is a pointer type
			if newParamFn != nil {
				param = newParamFn()
				err = r.getAll(param, inputReflectPlan)
			} else {
				param = tDefault
				//fmt.Print(param)
				err = r.getAll(&param, inputReflectPlan)
			}
			//if options.inputType.Kind() == reflect.Ptr {
			//	param = reflect.New(options.inputType.Elem()).Interface().(T)
			//	err = r.GetAll(param)
			//} else {
			//	err = r.GetAll(&param)
			//}

			var resp U
			if err == nil {

				var consumed bool
				if options.RequestHandler != nil {
					consumed, err = options.RequestHandler(r, param)
				}

				if err == nil && !consumed {
					for _, ext := range r.cache.extopts {
						if err == nil && !consumed && ext.RequestHandler != nil {
							consumed, err = ext.RequestHandler(r, options, param)
						}
					}
				}

				r.cache.input = param
				if err == nil && !consumed {
					resp, err = rw.call_internal(r, param, false)
					//consumed, resp, err = cachedHandleFunc(r, handlefunc, options, param)
					//if !consumed {
					//	resp, err = handlefunc(r, param)
					//}
				}

			}
			/*
				// AutoAuditLogger
				autoaudit := false
				switch r.config.Log.Audit.AutoAuditPolicy {
				case AutoAuditAlways:
					autoaudit = true
				case AutoAuditLogin:
					uid, _, _, _ := r.User()
					if uid != "" {
						autoaudit = true
					}
				}

				if options.AutoAudit || autoaudit {
					autoauditmsg := "autoaudit"
					if options.AutoAuditMsg != "" {
						autoauditmsg = options.AutoAuditMsg
					}
					if !isReallyNil(err) {
						r.Audit(autoauditmsg, zap.Any("error", err))
					} else {
						var respAny any = resp
						switch v := respAny.(type) {
						case []byte:
							if r.config.Log.Audit.AutoAuditBytesOutput {
								r.Audit(autoauditmsg, zap.Binary("output", v))
							} else {
								r.Audit(autoauditmsg)
							}
						case string:
							if r.config.Log.Audit.AutoAuditStringOutput {
								r.Audit(autoauditmsg, zap.String("output", v))
							} else {
								r.Audit(autoauditmsg)
							}
						default:
							r.Audit(autoauditmsg, zap.Any("output", v))
						}
					}
				}
			*/

			// Response
			if !isReallyNil(err) {
				for _, ext := range r.cache.extopts {
					if ext.ErrorHandler != nil {
						ok := ext.ErrorHandler(r, options, err)
						if ok {
							return
						}
					}
				}

				if options.ErrorHandler != nil {
					ok := options.ErrorHandler(r, err)
					if ok {
						return
					}
				}

				h := contentTypeHandlerThis //Map[options.ContentType]
				if h != nil {
					h.ErrorHandler(r, options, err)
					return
				}

				h = contentTypeHandlerHTML //Map[HTML]
				if h != nil {
					h.ErrorHandler(r, options, err)
					return
				}
				return
			}

			for _, ext := range r.cache.extopts {
				if ext.ResponseHandler != nil {
					ok := ext.ResponseHandler(r, options, resp)
					if ok {
						return
					}
				}
			}

			if options.ResponseHandler != nil {
				ok := options.ResponseHandler(r, resp)
				if ok {
					return
				}
			}

			h := contentTypeHandlerThis //Map[options.ContentType]
			if h != nil {
				h.ResponseHandler(r, options, resp)
				return
			}

			h = contentTypeHandlerHTML //Map[HTML]
			if h != nil {
				h.ResponseHandler(r, options, resp)
				return
			}
		},
	}

	if options.Name != "" {
		handlerMarshalMap[encodeHandlerName(options)] = rw
	}
	options.hasSelfDiscovery = hasSelfDiscovery(reflect.TypeOf(t).Elem())
	options.consumer = rw.job_consume

	typedHandlerList = append(typedHandlerList, rw)
	return rw
}

func IsAny[T any]() bool {
	var t *T
	var a *any
	tType := reflect.TypeOf(t).Elem()
	anyType := reflect.TypeOf(a).Elem()
	return tType == anyType
}

type GenericTypedHandler[T, U any, E error] struct {
	options    *HandlerOption
	handlefunc func(r *Request, input T) (output U, err E)
	handler    func(r *Request)

	// for SelfDiscovery
	Handler string `json:"handler"`
}

func (rw *GenericTypedHandler[T, U, E]) Options() *HandlerOption {
	return rw.options
}

func (rw *GenericTypedHandler[T, U, E]) Copy() TypedHandler {
	opt := *rw.options
	return &GenericTypedHandler[T, U, E]{
		options:    &opt,
		handlefunc: rw.handlefunc,
		handler:    rw.handler,
	}
}

func (rw *GenericTypedHandler[T, U, E]) Call(r *Request, input T) (output U, err error) {
	newR := *r // shallow copy (rは構造体)
	newR.memo = requestMemo{}
	opt := rw.options
	if opt != nil {
		newR.loggerWith = r.Logger().With(zap.String("caller", opt.Path)) // 区別しやすく
	}

	if !r.config.System.DisableValidator {
		if err := r.cache.validator.Struct(input); err != nil {
			var zeroU U
			return zeroU, err
		}
	}

	output, err = rw.call_internal(&newR, input, true)
	return
}

func (rw *GenericTypedHandler[T, U, E]) NewInputWithDefault() T {
	var t T
	t = NewRefOf[T](func(a any) {
		setDefault(a)
	})
	return t
}

func (rw *GenericTypedHandler[T, U, E]) HandleRequest(r *Request) {
	rw.handler(r)
}

func (rw *GenericTypedHandler[T, U, E]) HandleCall(r *Request, input any) (output any, err error) {
	var inT T
	inT, _ = input.(T)
	return rw.Call(r, inT)
}

var contentTypeHandlerMap map[string]*contentTypeHandler

type contentTypeHandler struct {
	responseHandler func(r *Request, options *HandlerOption, output any)
	errorHandler    func(r *Request, options *HandlerOption, err error)
}

func (c *contentTypeHandler) ResponseHandler(r *Request, options *HandlerOption, output any) {
	c.responseHandler(r, options, output)
}

func (c *contentTypeHandler) ErrorHandler(r *Request, options *HandlerOption, err error) {
	c.errorHandler(r, options, err)
}

func init() {
	contentTypeHandlerMap = make(map[string]*contentTypeHandler)
	contentTypeHandlerMap[HTML] = &contentTypeHandler{
		responseHandler: func(r *Request, options *HandlerOption, output any) {
			r.fiber.Status(options.ResponseStatusCode)
			switch v := output.(type) {
			case []byte:
				r.fiber.Write(v)
			case string:
				r.fiber.Write([]byte(v))
			default:
				if options.HTMLTemplate != "" {
					if options.parsedTemplate == nil {
						tmpl, err := template.New("html").Parse(options.HTMLTemplate)
						if err != nil {
							r.errorRedirect(options.ErrorStatusCode, err)
							return
						}
						options.parsedTemplate = tmpl
					}

					err := options.parsedTemplate.Execute(r.fiber, v)
					if err != nil {
						r.errorRedirect(options.ErrorStatusCode, err)
						return
					}
					return
				}

				if !isReallyNil(v) {
					r.fiber.Write([]byte(fmt.Sprint(v)))
				}
			}

		},
		errorHandler: func(r *Request, options *HandlerOption, err error) {
			if redir, ok := err.(FiberHandler); ok {
				redir.HandleFiber(r.fiber)
				return
			}
			r.errorRedirect(options.RedirectStatusCode, err)
		},
	}

	contentTypeHandlerMap[JSON] = &contentTypeHandler{
		responseHandler: func(r *Request, options *HandlerOption, output any) {
			if !options.NoWrapJSON {
				output = &APIResponse[any]{output}
			}
			buf, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				r.errorJSON(options.ErrorStatusCode, options.NoWrapJSON, options.eiserror, err)
				return
			}

			r.fiber.Status(options.ResponseStatusCode)
			r.fiber.Write(buf)
		},
		errorHandler: func(r *Request, options *HandlerOption, err error) {
			if redir, ok := err.(FiberHandler); ok {
				redir.HandleFiber(r.fiber)
				return
			}

			r.errorJSON(options.ErrorStatusCode, options.NoWrapJSON, options.eiserror, err)
		},
	}
}
