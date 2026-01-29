package allino

import (
	"bytes"
	"reflect"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var extensionList []extendable

type ExtOption struct {
	Info            *ExtInfo
	OnInit          func(s *Server) error
	OnHandlerInit   func(s *Server, opt *HandlerOption) error
	OnServe         func(s *Server) error
	OnShutdown      func(s *Server) error
	RequestHandler  func(r *Request, opt *HandlerOption, input any) error
	ResponseHandler func(r *Request, opt *HandlerOption, output any) (consumed bool)
	ErrorHandler    func(r *Request, opt *HandlerOption, err error) (consumed bool)
	CLICommands     []*cobra.Command
}

type extendable interface {
	ExtOption() ExtOption
	Update(setting []byte) error
}

type ExtInfo struct {
	Name string
}

type Extension[E, F any] struct {
	Info   *ExtInfo
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

func NewExtension[E, F any](info *ExtInfo, opt *ExtOption) *Extension[E, F] {
	var config E
	if opt == nil {
		opt = &ExtOption{}
	}
	if info == nil {
		info = &ExtInfo{
			Name: "unknown",
		}
	}
	ce := &Extension[E, F]{
		Info:   info,
		Config: &config,
		Option: opt,
	}
	extensionList = append(extensionList, ce)
	return ce
}
