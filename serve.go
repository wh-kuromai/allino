package allino

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/wh-kuromai/cryptino"
	"go.uber.org/zap"

	_ "embed"
)

type Config struct {
	ConfigBytes []byte `json:"-"`
	ConfigDir   string `json:"-"`
	ConfigFS    fs.FS  `json:"-"`
	AbsWorkDir  string `json:"-"`

	OnInit     func(s *Server) error       `json:"-"`
	OnServe    func(s *Server) error       `json:"-"`
	OnShutdown func(s *Server) error       `json:"-"`
	OnError    func(msg string, err error) `json:"-"`

	AppName     string `json:"appName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Bind        string `json:"bind"`
	NoWelcome   bool   `json:"nowelcome"`
	NoWrapJSON  bool   `json:"nowrapjson"`

	Routing      RoutingConfig      `json:"routing"`
	Login        LoginConfig        `json:"login"`
	Redis        RedisConfig        `json:"redis"`
	SQL          SQLConfig          `json:"sql"`
	Log          LogConfig          `json:"log"`
	Fiber        fiber.Config       `json:"fiber"`
	WebSocket    WebSocketConfig    `json:"websocket"`
	Https        HttpsConfig        `json:"https"`
	System       SystemConfig       `json:"system"`
	TrustedProxy TrustedProxyConfig `json:"trustedproxy"`

	Debug            bool      `json:"debug"`
	DisabledCommands []string  `json:"-"`
	Prefix           string    `json:"-"`
	StartAt          time.Time `json:"-"`
}

type SystemConfig struct {
	DisableValidator bool `json:"disable_validator"`
	DisableExtension bool `json:"disable_extension"`
}

type TrustedProxyConfig struct {
	TrustXForwardedFor bool `json:"trustXForwardedFor"`
	TrustXRequestID    bool `json:"trustXRequestID"`
}

type RoutingConfig struct {
	FallbackPaths []string `json:"fallbacks"`
	ErrorPath     string   `json:"error"`
	Err404Path    string   `json:"404error"`
}

type Server struct {
	Config    *Config
	Fiber     *fiber.App
	Logger    *zap.Logger
	Redis     redis.UniversalClient
	SQL       *sql.DB
	Cron      *cron.Cron
	Validator *validator.Validate

	yamlDecodeOption []yaml.DecodeOption
	yamlEncodeOption []yaml.EncodeOption

	extensions []extendable
	extopts    []ExtOption

	typedHandlerCache []TypedHandler
	optionsCache      []*HandlerOption
	//runAsPlugin bool
}

//go:embed appsetting_default.yaml
var settingDefault []byte

func NewServer(config *Config) (*Server, error) {
	s := &Server{
		Config: &Config{},
		//Router: httprouter.New(),
		Cron:      cron.New(),
		Validator: validator.New(),
	}

	//s.TypedRouter = &TypedRouter{
	//	//Router: s.Router,
	//	server: s,
	//}

	s.yamlDecodeOption = NewYAMLCustomDecodeOption()
	s.yamlDecodeOption = append(s.yamlDecodeOption, yaml.UseJSONUnmarshaler())
	s.yamlEncodeOption = NewYAMLCustomEncodeOption()

	if !s.Config.System.DisableExtension {
		s.extensions = extensionList
		for _, ext := range s.extensions {
			s.extopts = append(s.extopts, ext.ExtOption())
		}
	}

	s.Config.StartAt = time.Now()
	if s.Config.Prefix == "" {
		s.Config.Prefix = "allino"
	}

	err := s.Update(settingDefault, false)
	if err != nil {
		return nil, err
	}

	err = mergeStruct(s.Config, config)
	if err != nil {
		return nil, err
	}

	if s.Config.ConfigBytes != nil {
		err = s.Update(s.Config.ConfigBytes, false)
		if err != nil {
			return nil, err
		}
	}

	config_files := []string{
		s.filePrefix() + ".config.yaml",
		s.filePrefix() + ".config.enc",
	}

	secrets_files := []string{
		"secrets.config.json",
		"secrets.config.enc",
	}

	for _, config_file := range config_files {
		err = s.updateFS(config_file, false)
		if err != nil {
			return nil, err
		}
	}

	for _, secrets_file := range secrets_files {
		err = s.updateFS(secrets_file, true)
		if err != nil {
			return nil, err
		}
	}

	for _, config_file := range config_files {
		err = s.updateFile(config_file, false)
		if err != nil {
			return nil, err
		}
	}

	for _, secrets_file := range secrets_files {
		err = s.updateFile(secrets_file, true)
		if err != nil {
			return nil, err
		}
	}

	err = s.generateKeyIfNotExist()
	if err != nil {
		return nil, err
	}

	s.Config.Fiber.DisableStartupMessage = true
	s.Fiber = fiber.New(s.Config.Fiber)
	//s.Server, err = s.Config.Server.Setup()
	//if err != nil {
	//	return nil, err
	//}
	//
	var logmiddle fiber.Handler
	s.Logger, logmiddle, err = s.Config.Log.Setup(s.Cron)
	if err != nil {
		return nil, err
	}

	if logmiddle != nil {
		s.Fiber.Use(logmiddle)
	}

	err = s.Config.Login.setup()
	if err != nil {
		return nil, err
	}

	s.Redis, err = s.Config.Redis.connect()
	if err != nil {
		return nil, err
	}

	s.SQL, err = s.Config.SQL.connect()
	if err != nil {
		return nil, err
	}

	if len(s.Cron.Entries()) > 0 {
		s.Cron.Start()
	}

	if s.Config.OnInit != nil {
		err = s.Config.OnInit(s)
		if err != nil {
			return nil, fmt.Errorf("OnInit error: %w", err)
		}
	}

	for _, ext := range s.extopts {
		if ext.OnInit != nil {
			err = ext.OnInit(s)
			if err != nil {
				return nil, fmt.Errorf("OnInit error for Extension %s: %w", ext.Info.Name, err)
			}
		}
	}

	for _, th := range s.typedHandlerCache {
		opt := th.Options()
		if opt.OnInit != nil {
			err = opt.OnInit(s)
			if err != nil {
				return nil, fmt.Errorf("OnInit error for TypedHandler %s: %w", opt.Path, err)
			}
		}
	}

	return s, nil
}

func (s *Server) filePrefix() string {
	return strings.ToLower(s.Config.Prefix)
}

func (s *Server) envPrefix() string {
	return strings.ToUpper(s.Config.Prefix)
}

func NewTestServer(config *Config) *Server {
	s, err := NewServer(config)
	if err != nil {
		panic(fmt.Sprintf("NewTestServer error: %s", err))
	}
	s.RegisterAllTypedHandler()
	s.serveInitOnly()
	return s
}

func (s *Server) updateFS(filename string, secure bool) error {
	if s.Config.ConfigFS == nil {
		return nil
	}

	file, err := s.Config.ConfigFS.Open(filepath.Join(s.Config.ConfigDir, filename))
	if err != nil {
		return nil
	}

	buf, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return s.updateBuf(buf, filename, secure)
}

func (s *Server) updateFile(filename string, secure bool) error {
	buf, err := os.ReadFile(filepath.Join(s.Config.ConfigDir, filename))
	if err != nil {
		return nil
	}
	return s.updateBuf(buf, filename, secure)
}

func (s *Server) updateBuf(buf []byte, filename string, secure bool) (err error) {
	if strings.HasSuffix(filename, ".enc") {
		key := os.Getenv(s.envPrefix() + "_SECRET")
		if key == "" {
			return fmt.Errorf("encrypted config `%s` found but "+s.envPrefix()+"_SECRET not set", filename)
		}

		keybuf, err := base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			return err
		}
		if len(keybuf) != 32 {
			return fmt.Errorf("invalid key "+s.envPrefix()+"_SECRET: %s", key)
		}

		buf, err = cryptino.DecryptByGCM(keybuf, buf)
		if err != nil {
			return err
		}

	}
	return s.Update(buf, secure)
}

func (s *Server) Update(setting []byte, secure bool) error {
	decoder := yaml.NewDecoder(bytes.NewBuffer(setting), s.yamlDecodeOption...)
	err := decoder.Decode(s.Config)
	if err != nil {
		return err
	}

	if !secure {
		for _, configext := range s.extensions {
			err = configext.Update(setting)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) generateKeyIfNotExist() error {
	if !isReallyNil(s.Config.Login.PrivateKey) || !isReallyNil(s.Config.Login.PublicKey) {
		return nil
	}

	priv, err := cryptino.GenerateKey("ES256")
	if err != nil {
		return err
	}

	s.Config.Login.PrivateKey = priv
	return nil
}

func isReallyNil(value any) bool {
	if value == nil {
		return true
	}

	// reflect.ValueOf(value) が nil に対応しているかを確認
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func (s *Server) errorPrintln(msg string, err error) {
	if s.Config.OnError != nil {
		s.Config.OnError(msg, err)
	} else {
		fmt.Println(msg, err)
	}
}

func (s *Server) Serve() {
	var listener net.Listener
	var err error
	var isUnix bool

	addr := s.Config.Bind
	if strings.HasPrefix(addr, "unix:") {
		isUnix = true
		socketPath := strings.TrimPrefix(addr, "unix:")

		// 古いソケットがあったら削除（忘れると "address already in use"）
		os.Remove(socketPath)

		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			s.errorPrintln("Failed to listen on unix socket: ", err)
			return
		}
	} else {
		if s.Config.Https.Enabled() {
			var cert tls.Certificate
			cert, err = tls.LoadX509KeyPair(s.Config.Https.CertFile, s.Config.Https.KeyFile)
			if err != nil {
				log.Fatalf("failed to load cert: %v", err)
			}

			listener, err = tls.Listen("tcp", ":443", &tls.Config{
				Certificates: []tls.Certificate{cert},
			})
			if err != nil {
				log.Fatalf("Failed to listen on TCP:433: %v", err)
			}
		} else {
			listener, err = net.Listen("tcp", addr)
			if err != nil {
				s.errorPrintln("Failed to listen on TCP:"+addr+": ", err)
				return
			}
		}
	}

	protocol := "http"
	if s.Config.Https.Enabled() {
		protocol = "https"
	}
	bindURL := s.Config.Bind
	if strings.HasPrefix(bindURL, ":") {
		bindURL = protocol + "://localhost" + bindURL
	} else {
		bindURL = protocol + "://" + bindURL
	}

	if isUnix {
		protocol = "unix"
		bindURL = s.Config.Bind
	}

	if !s.Config.Log.Silent && !s.Config.NoWelcome {
		s.showWelcome(bindURL)
	}

	if s.Config.Debug {
		fmt.Println("WARNING: debug=true: DO NOT use this on production.")
		if s.Config.Log.Silent {
			fmt.Println("         log.silent was overridden to false due to debug mode")
			s.Config.Log.Silent = false
		}
		fmt.Println("")
	}

	if !s.Config.Log.Silent {
		//fmt.Println("Starting " + strings.ToUpper(protocol) + " server: '" + bindURL + "'")
		fmt.Println("")
		s.Logger.Info("server starting",
			zap.String("protocol", protocol),
			zap.String("bind", s.Config.Bind),
			zap.String("url", bindURL),
			zap.Time("startAt", s.Config.StartAt),
		)
	}

	for _, ext := range s.extopts {
		if ext.OnHandlerInit != nil {
			opts := s.RegisteredTypedHandlers()
			for _, opt := range opts {
				err = ext.OnHandlerInit(s, opt)
				if err != nil {
					s.errorPrintln(fmt.Sprintf("OnHandlerInit error for Extension `%s` to path `%s`: ", ext.Info.Name, opt.Path), err)
				}
			}
		}
	}

	if s.Config.OnServe != nil {
		err := s.Config.OnServe(s)
		if err != nil {
			s.errorPrintln("OnServe error: ", err)
		}
	}

	for _, ext := range s.extopts {
		if ext.OnServe != nil {
			err = ext.OnServe(s)
			if err != nil {
				s.errorPrintln(fmt.Sprintf("OnServe error for Extension `%s`: ", ext.Info.Name), err)
			}
		}
	}

	// Serve
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		err = s.Fiber.Listener(listener)
		if err != nil && err != http.ErrServerClosed {
			s.errorPrintln("ListenAndServe error: ", err)
		}
	}()

	<-quit

	if !s.Config.Log.Silent {
		fmt.Println("Shutting down server...")
		s.Logger.Info("server shutting down")
	}

	// 二段階終了検知用のセカンドチャンネル
	forceQuit := make(chan os.Signal, 1)
	signal.Notify(forceQuit, os.Interrupt)

	go func() {
		<-forceQuit // 2回目のCtrl+Cで強制終了
		fmt.Println("Force quitting...")
		os.Exit(1)
	}()

	if s.Config.OnShutdown != nil {
		err := s.Config.OnShutdown(s)
		if err != nil {
			s.errorPrintln("OnShutdown error: ", err)
		}
	}

	for _, ext := range s.extopts {
		if ext.OnShutdown != nil {
			err = ext.OnShutdown(s)
			if err != nil {
				s.errorPrintln(fmt.Sprintf("OnShutdown error for Extension `%s`: ", ext.Info.Name), err)
			}
		}
	}

	for _, th := range s.typedHandlerCache {
		opt := th.Options()
		if opt.OnShutdown != nil {
			err = opt.OnShutdown(s)
			if err != nil {
				s.errorPrintln(fmt.Sprintf("OnShutdown error for TypedHandler `%s`: ", opt.Path), err)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Fiber.ShutdownWithContext(ctx); err != nil {
		s.errorPrintln("Shutdown error: ", err)
	}

	if !s.Config.Log.Silent {
		fmt.Println("Server gracefully stopped")
		s.Logger.Info("server shutdown complete")
	}
}

func (s *Server) serveInitOnly() {
	for _, ext := range s.extopts {
		if ext.OnHandlerInit != nil {
			opts := s.RegisteredTypedHandlers()
			for _, opt := range opts {
				err := ext.OnHandlerInit(s, opt)
				if err != nil {
					s.errorPrintln(fmt.Sprintf("OnHandlerInit error for Extension `%s` to path `%s`: ", ext.Info.Name, opt.Path), err)
				}
			}
		}
	}

	if s.Config.OnServe != nil {
		err := s.Config.OnServe(s)
		if err != nil {
			s.errorPrintln("OnServe error: ", err)
		}
	}

	for _, ext := range s.extopts {
		if ext.OnServe != nil {
			err := ext.OnServe(s)
			if err != nil {
				s.errorPrintln(fmt.Sprintf("OnServe error for Extension `%s`: ", ext.Info.Name), err)
			}
		}
	}

}
