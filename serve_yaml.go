package allino

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/redis/go-redis/v9"
	"github.com/wh-kuromai/cryptino"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ByteSize int64

func (b ByteSize) Megabytes() int {
	return int(b / (1024 * 1024))
}

func NewYAMLCustomDecodeOption() []yaml.DecodeOption {

	opts := make([]yaml.DecodeOption, 0, 20)

	opt := yaml.CustomUnmarshaler(func(enc *zapcore.EncoderConfig, b []byte) error {
		encoderCfg := zap.NewProductionEncoderConfig()
		err := yaml.Unmarshal(b, encoderCfg)
		if err != nil {
			return err
		}
		*enc = encoderCfg
		return nil
	})
	opts = append(opts, opt)

	opt = yaml.CustomUnmarshaler(func(pubkey *cryptino.PublicKey, b []byte) error {
		pub, err := cryptino.UnmarshalJSONPublicKey(b)
		if err != nil {
			return err
		}
		*pubkey = pub
		return nil
	})
	opts = append(opts, opt)

	opt = yaml.CustomUnmarshaler(func(privkey *cryptino.PrivateKey, b []byte) error {
		priv, err := cryptino.UnmarshalJSONPrivateKey(b)
		if err != nil {
			return err
		}
		*privkey = priv
		return nil
	})
	opts = append(opts, opt)

	opt = yaml.CustomUnmarshaler(func(ptr *time.Duration, b []byte) error {
		// 空文字はゼロ値
		if len(b) == 0 {
			*ptr = 0
			return nil
		}

		n, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err == nil {
			*ptr = time.Duration(n) * time.Second
			return nil
		}

		parsed, err := time.ParseDuration(strings.TrimSpace(string(b)))
		if err != nil {
			return err
		}
		*ptr = parsed
		return nil
	})
	opts = append(opts, opt)

	opt = yaml.CustomUnmarshaler(func(ptr *ByteSize, b []byte) error {
		s := strings.TrimSpace(strings.ToUpper(string(b)))
		if s == "" {
			*ptr = 0
			return nil
		}

		n, err := strconv.Atoi(s)
		if err == nil {
			*ptr = ByteSize(n)
			return nil
		}

		// 末尾の単位を判別（B/KB/MB/GB/TB、KiB/MiB... も対応）
		unitMul := map[string]int64{
			"B":  1,
			"KB": 1000, "MB": 1000 * 1000, "GB": 1000 * 1000 * 1000, "TB": 1000 * 1000 * 1000 * 1000,
			"KIB": 1024, "MIB": 1024 * 1024, "GIB": 1024 * 1024 * 1024, "TIB": 1024 * 1024 * 1024 * 1024,
		}

		// 数字部分と単位をざっくり分離
		i := len(s)
		for i > 0 && (s[i-1] < '0' || s[i-1] > '9') {
			i--
		}
		numPart := strings.TrimSpace(s[:i])
		unitPart := strings.TrimSpace(s[i:])
		if unitPart == "" {
			unitPart = "B"
		}
		mul, ok := unitMul[unitPart]
		if !ok {
			return fmt.Errorf("unknown size unit: %q", unitPart)
		}
		v, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			return fmt.Errorf("invalid size number: %w", err)
		}
		*ptr = ByteSize(int64(v * float64(mul)))
		return nil
	})
	opts = append(opts, opt)

	opts = append(opts, yaml.UseJSONUnmarshaler())
	return opts
}

func NewYAMLCustomEncodeOption() []yaml.EncodeOption {
	opts := make([]yaml.EncodeOption, 0, 20)

	opt := yaml.CustomMarshaler(func(redis.Options) ([]byte, error) {
		return nil, nil
	})
	opts = append(opts, opt)
	return opts
}
