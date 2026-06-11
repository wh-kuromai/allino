package bench

import (
	"reflect"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type NoTag struct {
	Name string
	Age  int
	Mail string
}

type WithTag struct {
	Name string `validate:"required"`
	Age  int    `validate:"gte=0,lte=150"`
	Mail string `validate:"email"`
}

var tagCache sync.Map

func HasValidationTag(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if v, ok := tagCache.Load(t); ok {
		return v.(bool)
	}

	has := false

	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("validate") != "" {
				has = true
				break
			}
		}
	}

	tagCache.Store(t, has)
	return has
}

func ValidateWithPreCheck(v any) error {
	t := reflect.TypeOf(v)

	if !HasValidationTag(t) {
		return nil
	}

	return validate.Struct(v)
}

func BenchmarkValidate_NoTag(b *testing.B) {
	v := NoTag{
		Name: "foo",
		Age:  20,
		Mail: "foo@example.com",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = validate.Struct(v)
	}
}

func BenchmarkValidate_NoTag_PreCheck(b *testing.B) {
	v := NoTag{
		Name: "foo",
		Age:  20,
		Mail: "foo@example.com",
	}

	// warmup
	_ = HasValidationTag(reflect.TypeOf(v))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ValidateWithPreCheck(v)
	}
}

func BenchmarkValidate_WithTag(b *testing.B) {
	v := WithTag{
		Name: "foo",
		Age:  20,
		Mail: "foo@example.com",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = validate.Struct(v)
	}
}

func BenchmarkValidate_WithTag_PreCheck(b *testing.B) {
	v := WithTag{
		Name: "foo",
		Age:  20,
		Mail: "foo@example.com",
	}

	// warmup
	_ = HasValidationTag(reflect.TypeOf(v))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = ValidateWithPreCheck(v)
	}
}
