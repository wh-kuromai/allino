package allino

import (
	"fmt"
	"reflect"
)

/*
// MergeStructByJSON merges two structs using JSON round-trip and override strategy
func mergeStructByJSON[T any](base, override T) (T, error) {
	var baseMap, overrideMap map[string]interface{}

	baseBytes, err := json.Marshal(base)
	if err != nil {
		return base, fmt.Errorf("marshal base failed: %w", err)
	}
	overrideBytes, err := json.Marshal(override)
	if err != nil {
		return base, fmt.Errorf("marshal override failed: %w", err)
	}

	if err := json.Unmarshal(baseBytes, &baseMap); err != nil {
		return base, fmt.Errorf("unmarshal base map failed: %w", err)
	}
	if err := json.Unmarshal(overrideBytes, &overrideMap); err != nil {
		return base, fmt.Errorf("unmarshal override map failed: %w", err)
	}

	// override fields
	for k, v := range overrideMap {
		baseMap[k] = v
	}

	// back to target struct
	mergedBytes, err := json.Marshal(baseMap)
	if err != nil {
		return base, fmt.Errorf("re-marshal merged map failed: %w", err)
	}

	var result T
	if err := json.Unmarshal(mergedBytes, &result); err != nil {
		return base, fmt.Errorf("unmarshal to result failed: %w", err)
	}

	return result, nil
}
*/
/*
type RawMessage []byte

func (m RawMessage) MarshalJSON() ([]byte, error) { return m.marshal() }
func (m RawMessage) MarshalYAML() ([]byte, error) { return m.marshal() }

func (m RawMessage) marshal() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

func (m *RawMessage) UnmarshalJSON(data []byte) error { return m.unmarshal(data) }
func (m *RawMessage) UnmarshalYAML(data []byte) error { return m.unmarshal(data) }

func (m *RawMessage) unmarshal(data []byte) error {
	if m == nil {
		return errors.New("RawMessage: unmarshal on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}
*/

func mergeStruct(dst, src interface{}) error {
	if isReallyNil(src) {
		return nil
	}
	vDst := reflect.ValueOf(dst)
	vSrc := reflect.ValueOf(src)

	// dstは必ずアドレス可能な構造体
	if vDst.Kind() != reflect.Ptr {
		return fmt.Errorf("dst must be a pointer")
	}

	vDstElem := indirect(vDst)
	vSrcElem := indirect(vSrc)

	if vDstElem.Kind() != reflect.Struct || vSrcElem.Kind() != reflect.Struct {
		return fmt.Errorf("both dst and src must be or point to struct")
	}

	return mergeValue(vDstElem, vSrcElem)
}

func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			// 必要なら nil ポインタも新しく作る（安全のため無効化可能）
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

func mergeValue(dst, src reflect.Value) error {
	//t := dst.Type()

	for i := 0; i < dst.NumField(); i++ {
		//field := t.Field(i)
		fDst := dst.Field(i)
		fSrc := src.Field(i)

		if !fDst.CanSet() {
			continue
		}

		switch fDst.Kind() {
		case reflect.Struct:
			if err := mergeValue(fDst, fSrc); err != nil {
				return err
			}
		case reflect.Ptr:
			if fDst.IsNil() && !fSrc.IsNil() {
				// dstがnilなら、srcの値をコピー
				fDst.Set(fSrc)
			} else if !fDst.IsNil() && !fSrc.IsNil() && fDst.Elem().Kind() == reflect.Struct {
				// 両方非nilかつStructなら再帰
				if err := mergeValue(fDst.Elem(), fSrc.Elem()); err != nil {
					return err
				}
			}
		default:
			if !isZeroValue(fSrc) {
				fDst.Set(fSrc)
			}
		}
	}

	return nil
}

func isZeroValue(v reflect.Value) bool {
	return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}

//func shallowCopyMap[K comparable, V any](src map[K]V) map[K]V {
//	dst := make(map[K]V, len(src))
//	for k, v := range src {
//		dst[k] = v
//	}
//	return dst
//}
