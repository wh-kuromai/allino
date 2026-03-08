package allino

import (
	"bytes"
	"reflect"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var extensionList []extendable

type ExtInfo struct {
	Name string
}

type ExtOption struct {
	ExtInfo

	OnInit          func(s *Server, virtual *Request) error
	OnHandlerInit   func(s *Server, virtual *Request, opt *HandlerOption) error
	OnServe         func(s *Server, virtual *Request) error
	OnShutdown      func(s *Server, virtual *Request) error
	RequestHandler  func(r *Request, opt *HandlerOption, input any) (consumed bool, err error)
	ResponseHandler func(r *Request, opt *HandlerOption, output any) (consumed bool)
	ErrorHandler    func(r *Request, opt *HandlerOption, err error) (consumed bool)
	CLICommands     []*cobra.Command
}

type extendable interface {
	ExtOption() ExtOption
	Update(setting []byte) error
}

type Extension[E, F any] struct {
	Option *ExtOption
	Config *E
}

func (c Extension[E, F]) HandlerOptionExt(opt *HandlerOption) (*F, bool) {
	if opt == nil {
		return nil, false
	}
	if opt.exts == nil {
		opt.exts = &sync.Map{}
	}
	t := reflect.TypeOf((*F)(nil)).Elem()
	v, ok := opt.exts.Load(t)
	if !ok {
		v = handlerExtEntry{reflect.New(t).Interface(), false}
		opt.exts.Store(t, v)
	}
	return v.(handlerExtEntry).value.(*F), v.(handlerExtEntry).isUserSet
}

func (c Extension[E, F]) ExtOption() ExtOption {
	return *c.Option
}
func (c Extension[E, F]) Update(setting []byte) error {
	decoder := yaml.NewDecoder(bytes.NewBuffer(setting), yamlDecodeOption...)
	return decoder.Decode(c.Config)
}

func NewExtension[E, F any](name string, opt *ExtOption) *Extension[E, F] {
	var config E
	if opt == nil {
		opt = &ExtOption{}
	}
	opt.ExtInfo.Name = name
	ce := &Extension[E, F]{
		Config: &config,
		Option: opt,
	}
	extensionList = append(extensionList, ce)
	return ce
}
