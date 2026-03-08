package allino

import "reflect"

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

func NewRefOf[T any](f func(any)) T {
	tType := reflect.TypeOf((*T)(nil)).Elem()

	// T == any の場合
	if tType.Kind() == reflect.Interface && tType.NumMethod() == 0 {
		var zero T
		return zero
	}

	if tType.Kind() == reflect.Ptr {
		v := reflect.New(tType.Elem()).Interface()
		f(v)
		return v.(T)
	}
	ptr := reflect.New(tType)
	f(ptr.Interface())
	return ptr.Elem().Interface().(T)
}

/*
func NewErrorOf[T error](f func(T) error) error {
	var zero T
	e := f(zero)

	if isReallyNil(e) {
		return nil
	}
	return e
}
*/
