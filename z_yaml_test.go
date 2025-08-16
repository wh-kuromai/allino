package allino_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

type ByteSize int64

type TestYamlConfig struct {
	Int int           `yaml:"int"`
	Dur time.Duration `yaml:"dur"`
}

func TestYaml(t *testing.T) {
	var data = `
int: 89MB
dur: 1h12m
`
	opts := make([]yaml.DecodeOption, 0, 20)

	opt := yaml.CustomUnmarshaler(func(ptr *time.Duration, b []byte) error {
		// 空文字はゼロ値
		if len(b) == 0 {
			*ptr = 0
			return nil
		}
		parsed, err := time.ParseDuration(string(b))
		if err != nil {
			return err
		}
		*ptr = parsed
		return nil
	})
	opts = append(opts, opt)

	opt = yaml.CustomUnmarshaler(func(ptr *int, b []byte) error {
		s := strings.TrimSpace(strings.ToUpper(string(b)))
		if s == "" {
			*ptr = 0
			return nil
		}

		n, err := strconv.Atoi(s)
		if err == nil {
			*ptr = n
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
		*ptr = int(int64(v * float64(mul)))
		return nil
	})
	opts = append(opts, opt)

	var cfg TestYamlConfig
	if err := yaml.UnmarshalWithOptions([]byte(data), &cfg, opts...); err != nil {
		fmt.Println(cfg)
	}
	fmt.Println("_----_", cfg)
	fmt.Println("_----_", cfg.Dur.Minutes())
}
