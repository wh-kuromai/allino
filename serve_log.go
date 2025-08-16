package allino

import (
	"io"
	"math"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/natefinch/lumberjack"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogConfig struct {
	Silent      bool              `json:"silent"`
	NoRequestID bool              `json:"norequestid"`
	Zap         ZapConfig         `json:"zap"`
	Audit       AuditConfig       `json:"audit"`
	AccessLog   []LogOutputConfig `json:"accesslog"`
	ErrorLog    []LogOutputConfig `json:"errorlog"`
}

type AutoAuditPolicy int32

const (
	AutoAuditNever AutoAuditPolicy = iota
	AutoAuditLogin
	AutoAuditAlways
)

type AuditConfig struct {
	AutoAuditPolicy       AutoAuditPolicy `json:"autoaudit_policy,omitempty"`
	AutoAuditBytesOutput  bool            `json:"autoaudit_bytes_output,omitempty"`
	AutoAuditStringOutput bool            `json:"autoaudit_string_output,omitempty"`
}

type ZapConfig struct {
	AddCaller     bool    `json:"addcaller,omitempty"`
	AddCallerSkip int     `json:"addcallerskip,omitempty"`
	AddStacktrace *string `json:"addstacktrace,omitempty"`
}

// LogOutputConfig は Zap ロガーの出力先やエンコーダ設定を定義する構造体です。
//
// ・ログレベル、出力フォーマット（JSON / Console）などを指定可能です。
// ・標準出力・標準エラー・ファイル出力に対応し、lumberjack によるローテーションも可能です。
// ・EncoderConfig フィールドは map[string]interface{} として柔軟に受け取り、
//   JSON 経由で zapcore.EncoderConfig に変換されます。
//   - ただし、EncodeTime や EncodeLevel など関数型フィールドは変換処理で補完される必要があります。
//   - たとえば "encodeTime": "iso8601" や "encodeLevel": "capital" のように記述します。
//
// この構成により、設定ファイルから詳細な Zap のログ形式制御が可能になります。

type LogOutputConfig struct {
	To            string                 `json:"to,omitempty"` // "stdout", "stderr", "file" など
	Level         *string                `json:"loglevel,omitempty"`
	Format        *string                `json:"format,omitempty"`
	EncoderConfig *zapcore.EncoderConfig `json:"zap_encoderconfig,omitempty"`

	Path string `json:"path,omitempty"`
	//RotateCron *string            `json:"rotatecron,omitempty"`
	//Rotate     *lumberjack.Logger `json:"rotate,omitempty"`

	Rotate *LogRotateConfig `json:"rotate,omitempty"`

	entryID cron.EntryID `json:"-"`
}

type LogRotateConfig struct {
	Cron       *string       `json:"cron,omitempty"`
	Filename   string        `json:"filename" yaml:"filename"`
	MaxSize    ByteSize      `json:"maxsize" yaml:"maxsize"`
	MaxAge     time.Duration `json:"maxage" yaml:"maxage"`
	MaxBackups int           `json:"maxbackups" yaml:"maxbackups"`
	LocalTime  bool          `json:"localtime" yaml:"localtime"`
	Compress   bool          `json:"compress" yaml:"compress"`

	logger *lumberjack.Logger
}

func (cfg LogRotateConfig) ToLogger() *lumberjack.Logger {
	if cfg.logger != nil {
		return cfg.logger
	}

	cfg.logger = &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize.Megabytes(),                   // MB単位に変換
		MaxAge:     int(math.Ceil(cfg.MaxAge.Hours() / 24.0)), // 日数に変換
		MaxBackups: cfg.MaxBackups,
		LocalTime:  cfg.LocalTime,
		Compress:   cfg.Compress,
	}
	return cfg.logger
}

func (s *LogConfig) Setup(cn *cron.Cron) (*zap.Logger, fiber.Handler, error) {
	var logger *zap.Logger
	errfn := func(err error) {
		if logger != nil {
			logger.Error("lumberjack.Logger Rotate error", zap.Error(err))
		}
	}

	cores, err := s.loadErrLogConfigAll(s.ErrorLog, cn, errfn)
	if err != nil {
		return nil, nil, err
	}

	core := zapcore.NewTee(cores...)

	var opt []zap.Option
	if s.Zap.AddCaller {
		opt = append(opt, zap.AddCaller())
	}

	if s.Zap.AddCallerSkip != 0 {
		opt = append(opt, zap.AddCallerSkip(s.Zap.AddCallerSkip))
	}

	if s.Zap.AddStacktrace != nil {
		level, err := zapcore.ParseLevel(*s.Zap.AddStacktrace)
		if err != nil {
			return nil, nil, err
		}
		opt = append(opt, zap.AddStacktrace(level))
	}

	logger = zap.New(core, opt...)

	handler, err := s.loadAccessLogConfigAll(s.AccessLog, cn, errfn)
	if err != nil {
		return nil, nil, err
	}
	return logger, handler, nil

}

func (s *LogConfig) loadErrLogConfigAll(c []LogOutputConfig, cn *cron.Cron, errfn func(error)) ([]zapcore.Core, error) {
	if c == nil {
		return nil, nil
	}
	var lhs []zapcore.Core
	for _, cc := range c {
		lh, err := s.loadErrLogConfig(cc, cn, errfn)
		if err != nil {
			return nil, err
		}

		lhs = append(lhs, lh)
	}

	return lhs, nil
}

func (s *LogConfig) loadErrLogConfig(c LogOutputConfig, cn *cron.Cron, errfn func(error)) (core zapcore.Core, err error) {
	w, err := s.logWriter(c, cn, errfn)
	if err != nil {
		return nil, err
	}

	writer := zapcore.AddSync(w)

	var encoderCfg zapcore.EncoderConfig
	if c.EncoderConfig == nil {
		encoderCfg = zap.NewProductionEncoderConfig()
	} else {
		encoderCfg = *c.EncoderConfig
	}

	var encoder zapcore.Encoder
	if c.Format != nil && *c.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	var level zapcore.Level
	if c.Level != nil {
		level, err = zapcore.ParseLevel(*c.Level)
		if err != nil {
			return nil, err
		}
	} else {
		level = zapcore.InfoLevel // デフォルト
	}
	core = zapcore.NewCore(encoder, writer, level)
	return
}

// Web
func (s *LogConfig) loadAccessLogConfigAll(c []LogOutputConfig, cn *cron.Cron, errfn func(error)) (fiber.Handler, error) {
	if c == nil {
		return nil, nil
	}

	writers := make([]io.Writer, 0, 10)
	for _, cc := range c {
		w, err := s.logWriter(cc, cn, errfn)
		if err != nil {
			return nil, err
		}

		writers = append(writers, w)
	}

	if len(writers) > 0 {
		w := io.MultiWriter(writers...)
		middle := logger.New(logger.Config{
			Output:     w,
			Format:     "${ip} - - [${time}] \"${method} ${path} ${protocol}\" ${status} ${bytes} \"${referer}\" \"${ua}\"\n",
			TimeFormat: "02/Jan/2006:15:04:05 -0700", // Apache形式に近いタイムスタンプ
			TimeZone:   "Local",
		})
		return middle, nil
	}

	return nil, nil
}

func (s *LogConfig) logWriter(c LogOutputConfig, cn *cron.Cron, errfn func(error)) (io.Writer, error) {
	var w io.Writer
	switch c.To {
	case "file":
		if c.Rotate != nil {
			if c.Rotate.Cron != nil {
				eid, err := cn.AddFunc(*c.Rotate.Cron, func() {
					err := c.Rotate.ToLogger().Rotate()
					if err != nil {
						errfn(err)
					}
				})
				c.entryID = eid

				if err != nil {
					return nil, err
				}
			}

			w = c.Rotate.ToLogger()
		} else {
			var err error
			w, err = os.OpenFile(c.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, err
			}
		}

	case "stdout":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	}

	return w, nil
}
