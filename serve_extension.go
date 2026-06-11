package allino

import (
	"reflect"
	"sync"

	"github.com/spf13/cobra"
	"github.com/wh-kuromai/cryptino"
)

var extensionList []extendable

type ExtInfo struct {
	Name string
}

type ExtOption struct {
	ExtInfo

	SQLSchema       func(driver string) string
	OnInit          func(s *Server, virtual *Request) error
	OnHandlerInit   func(s *Server, virtual *Request, opt *HandlerOption) error
	OnServe         func(s *Server, virtual *Request) error
	OnShutdown      func(s *Server, virtual *Request) error
	OnAuthZ         func(r *Request, jwt *cryptino.JSONWebToken) (*cryptino.JSONWebToken, error)
	OnInjection     func(r *Request, params []*InjectionTarget) error
	RequestHandler  func(r *Request, opt *HandlerOption, input any) (consumed bool, err error)
	ResponseHandler func(r *Request, opt *HandlerOption, output any) (consumed bool)
	ErrorHandler    func(r *Request, opt *HandlerOption, err error) (consumed bool)
	CLICommands     []*cobra.Command

	//IsCallTarget func(opt *HandlerOption) bool
	//OnCall       func(r *Request, input any, fromcall bool) (output any, err error)
}

type InjectionTarget struct {
	Input     string
	Extension string
	Action    string
	Reference any
}

type extendable interface {
	ExtOption() *ExtOption
	Update(setting []byte) error
	initConfig(extconfig any)
}

type Extension[E, F any] struct {
	Option *ExtOption
	Config *E

	eIsAny bool
	fIsAny bool
}

func (c *Extension[E, F]) HandlerOptionExt(opt *HandlerOption) (F, bool) {
	var zeroF F
	if opt == nil {
		return zeroF, false
	}
	if opt.exts == nil {
		opt.exts = &sync.Map{}
	}

	t := reflect.TypeOf((*F)(nil)).Elem()
	v, ok := opt.exts.Load(t)
	if ok {
		he, ok2 := v.(handlerExtEntry)
		if ok2 {
			hev, ok3 := he.value.(F)
			if ok3 {
				return hev, he.isUserSet
			}
		}
	}

	if t.Kind() == reflect.Ptr {
		if !ok {
			vv := reflect.New(t).Interface()
			hev, ok3 := vv.(F)
			if ok3 {
				opt.exts.Store(t, handlerExtEntry{reflect.New(t).Interface(), false})
				return hev, false
			}
		}
	}

	return zeroF, false
}

func (c *Extension[E, F]) ExtOption() *ExtOption {
	return c.Option
}

func (c *Extension[E, F]) initConfig(extconfig any) {
	if c.eIsAny {
		return
	}

	extc, ok := extconfig.(*E)
	if ok {
		c.Config = extc
		setDefault(c.Config)
		return
	}

	extcd, ok := extconfig.(E)
	if ok {
		c.Config = &extcd
		setDefault(c.Config)
	}
}

func (c *Extension[E, F]) Update(setting []byte) error {
	if c.eIsAny {
		return nil
	}

	//decoder := yaml.NewDecoder(bytes.NewBuffer(setting)) //, yamlDecodeOption...)
	//return decoder.Decode(c.Config)

	YAMLPathUnmarshal(setting, "$."+c.Option.Name, c.Config)
	return nil
}

func NewExtension[CONF, OPT any](name string, opt *ExtOption) *Extension[CONF, OPT] {
	var config CONF
	if opt == nil {
		opt = &ExtOption{}
	}
	opt.ExtInfo.Name = name
	ce := &Extension[CONF, OPT]{
		Config: &config,
		Option: opt,
	}

	tEType := reflect.TypeOf((*CONF)(nil)).Elem()
	tFType := reflect.TypeOf((*OPT)(nil)).Elem()
	anyType := reflect.TypeOf((*any)(nil)).Elem()

	if tEType == anyType {
		ce.eIsAny = true
	}
	if tFType == anyType {
		ce.fIsAny = true
	}

	extensionList = append(extensionList, ce)
	return ce
}
