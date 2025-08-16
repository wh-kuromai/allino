package allino

import (
	"errors"
	"reflect"
	"time"
)

var (
	errInvalidType = errors.New("not a struct pointer")
)

const defaultTag = "default"

// var tTime = reflect.TypeOf(time.Time{})
var tDuration = reflect.TypeOf(time.Duration(0))

func setDefault(ptr interface{}) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errInvalidType
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errInvalidType
	}
	return setDefaultStruct(rv)
}

func setDefaultStruct(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		// 非公開はスキップ
		if sf.PkgPath != "" {
			continue
		}

		if !fv.IsZero() {
			continue
		}

		// time.Time は struct だが特別扱い
		if sf.Type == tTime {
			if dv, ok := sf.Tag.Lookup(defaultTag); ok && dv != "-" {
				if fv.IsZero() {
					if tm, err := parseTimeSafe(dv, time.UTC); err != nil {
						fv.Set(reflect.ValueOf(tm))
					}
				}
			}
			continue
		}

		// ネスト struct は再帰（但し time.Time は上で除外済み）
		if sf.Type.Kind() == reflect.Struct {
			if err := setDefaultStruct(fv); err != nil {
				return err
			}
			continue
		}

		// 通常の default 適用
		if dv, ok := sf.Tag.Lookup(defaultTag); ok && dv != "-" {

			ispointer := false
			basetyp := sf.Type
			if sf.Type.Kind() == reflect.Ptr {
				ispointer = true
				basetyp = basetyp.Elem()
			}

			setByReflect(dv, ispointer, basetyp, fv)
		}
	}
	return nil
}
