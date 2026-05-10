package allino

import (
	"encoding/json"
	"reflect"
)

type genericHandlerJSON struct {
	Handler string `json:"handler"`
	Version string `json:"version"`
}

func (rw *GenericTypedHandler[T, U, E]) MarshalJSON() ([]byte, error) {
	return json.Marshal(&genericHandlerJSON{
		Handler: encodeHandlerName(rw.options),
		Version: handlerVersion(rw.options),
	})
}

var handlerMarshalMap = make(map[string]any)

func (rw *GenericTypedHandler[T, U, E]) FindMaster() any {
	return handlerMarshalMap[rw.Handler]
}

type SelfDiscovery interface {
	FindMaster() any
}

var ErrTypeMismatch = NewError("type mismatch")

// FillMasterObjects は target 内の SelfDiscovery を実装したフィールドを走査し、
// FindMasterObject() の結果で値を更新します。
func fillSelfDiscovery(target any) error {
	v := reflect.ValueOf(target)

	// target がポインタでない場合は書き換えができないため、ポインタであることを確認
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}

	// ポインタの中身（構造体）を取得
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		// 公開フィールド（Exported）でない場合はスキップ
		if !field.CanSet() {
			continue
		}

		// 1. そのフィールド自体（またはそのポインタ）が SelfDiscovery を実装しているか確認
		// アドレスが取れる場合（Addr()）を考慮することで、ポインタレシーバのメソッドも拾えるようにします
		if field.CanAddr() {
			addr := field.Addr().Interface()
			if sd, ok := addr.(SelfDiscovery); ok {
				master := sd.FindMaster()
				if master != nil {
					masterVal := reflect.ValueOf(master)
					// フィールドの型と、代入しようとしている値の型に互換性があるか確認
					if masterVal.Type().AssignableTo(field.Type()) {
						field.Set(masterVal)
					} else {
						// 必要に応じてログを出したり、エラーを返したりする
						return ErrTypeMismatch
					}
				}
			}
		}

		// 2. フィールドが構造体、または構造体へのポインタの場合、再帰的に中身をチェック
		// フィールドがポインタの場合は Elem() で実体を見る
		deepField := field
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			deepField = field.Elem()
		}

		if deepField.Kind() == reflect.Struct {
			// 再帰的に呼び出し
			if field.Kind() == reflect.Ptr {
				fillSelfDiscovery(field.Interface())
			} else if field.CanAddr() {
				fillSelfDiscovery(field.Addr().Interface())
			}
		}
	}
	return nil
}

var (
	sdType = reflect.TypeOf((*SelfDiscovery)(nil)).Elem()
)

func hasSelfDiscovery(t reflect.Type) bool {

	// 1. その型自体、またはそのポインタ型がインターフェースを実装しているかチェック
	if t.Implements(sdType) || reflect.PtrTo(t).Implements(sdType) {
		return true
	}

	// 2. 複合型（ポインタ、スライス、マップ）の場合は、その中身をチェック
	switch t.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		if hasSelfDiscovery(t.Elem()) {
			return true
		}

	case reflect.Struct:
		// 3. 構造体の場合は全フィールドをスキャン
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// フィールドの型を再帰的にチェック
			if hasSelfDiscovery(field.Type) {
				return true
			}
		}
	}

	// 何も見つからなかった場合
	return false
}
