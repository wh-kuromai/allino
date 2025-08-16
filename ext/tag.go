package ext

import (
	"errors"
	"reflect"
	"slices"
)

type fieldPlan struct {
	tags      []string // len(tags) == tagSize, fieldPlan[tagKind] = field.Tag.LookUp(tagKind.String())
	tagoks    []bool
	name      string
	typ       reflect.Type
	basetyp   reflect.Type
	kind      reflect.Kind
	ispointer bool
	index     []int               // reflect.StructField.Index (親からのパス)
	sf        reflect.StructField // タグ等のメタもここから取る
	child     *ReflectPlan
	local     any
}

type ReflectPlan struct {
	taglist []string
	typ     reflect.Type
	fields  []*fieldPlan
}

func NewReflectPlan(t reflect.Type, taglist []string, initlocal ...func(name string, tagvalues []string, tagsok []bool) any) *ReflectPlan {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]*fieldPlan, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// 非公開フィールドはスキップ（タグ付けされていても set できないため）
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}

		fp := &fieldPlan{
			tags:      make([]string, len(taglist)),
			tagoks:    make([]bool, len(taglist)),
			name:      sf.Name,
			typ:       sf.Type,
			kind:      sf.Type.Kind(),
			index:     sf.Index,
			sf:        sf,
			ispointer: sf.Type.Kind() == reflect.Ptr,
		}

		if fp.ispointer {
			fp.basetyp = sf.Type.Elem()
		} else {
			fp.basetyp = sf.Type
		}

		for k := 0; k < len(taglist); k++ {
			tagName := taglist[k]
			if v, ok := sf.Tag.Lookup(tagName); ok {
				fp.tags[k] = v
				fp.tagoks[k] = true
			}
		}

		if fp.basetyp.Kind() == reflect.Struct && !fp.ispointer {
			fp.child = NewReflectPlan(fp.basetyp, taglist, initlocal...)
		}

		if len(initlocal) > 0 {
			fp.local = initlocal[0](fp.name, fp.tags, fp.tagoks)
		}

		fields = append(fields, fp)
	}

	return &ReflectPlan{
		taglist: taglist,
		typ:     t,
		fields:  fields,
	}
}

type FieldAccess[T any] struct {
	tagidx          int
	v               reflect.Value // 実フィールド（settable想定）
	fp              *fieldPlan    // 事前解析済みメタ
	structTag       reflect.StructTag
	selectedTagName string
}

func (f *FieldAccess[T]) Name() string {
	if f == nil || f.fp == nil || f.tagidx < 0 || f.tagidx >= len(f.fp.tags) {
		return ""
	}
	return f.fp.name
}

func (f *FieldAccess[T]) Tag() string {
	if f == nil || f.fp == nil || f.tagidx < 0 || f.tagidx >= len(f.fp.tags) {
		return ""
	}
	return f.fp.tags[f.tagidx]
}

func (f *FieldAccess[T]) SetLocal(v any) {
	f.fp.local = v
}

func (f *FieldAccess[T]) GetLocal() any {
	return f.fp.local
}

func (f *FieldAccess[T]) Lookup(tag string) (string, bool) {
	if f == nil {
		return "", false
	}
	return f.structTag.Lookup(tag)
}

func (f *FieldAccess[T]) Set(value T) {
	if f == nil {
		return
	}
	fv := f.v
	if !fv.IsValid() || !fv.CanSet() {
		return
	}
	fv.Set(reflect.ValueOf(value))
}

func (f *FieldAccess[T]) Get() T {
	var zero T
	if f == nil {
		return zero
	}
	fv := f.v
	if !fv.IsValid() {
		return zero
	}
	return fv.Interface().(T) // 必要なら堅牢版に差し替え
}

var (
	ErrNotStruct       = errors.New("not struct or struct pointer")
	ErrNotRegisterdTag = errors.New("not pre-registered tag")
)

func ExecutePlan[T any](r *ReflectPlan, tag string, target any, callback func(f FieldAccess[T]) error) error {
	if r == nil || target == nil || callback == nil || tag == "" {
		return ErrNotStruct
	}

	tagidx := slices.Index(r.taglist, tag)
	if tagidx < 0 {
		return ErrNotRegisterdTag // 未登録タグ
	}

	root := reflect.ValueOf(target)
	if root.Kind() == reflect.Ptr {
		root = root.Elem()
	}
	if root.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	tt := reflect.TypeOf((*T)(nil)).Elem()

	var walk func(rp *ReflectPlan, cur reflect.Value) error
	walk = func(rp *ReflectPlan, cur reflect.Value) error {
		if rp == nil || !cur.IsValid() || cur.Kind() != reflect.Struct {
			return ErrNotStruct
		}
		for _, fp := range rp.fields {
			fv := cur.FieldByIndex(fp.index)
			if !fv.IsValid() {
				continue
			}
			// 非ポインタのネスト struct は再帰
			if fp.child != nil && fv.Kind() == reflect.Struct {
				walk(fp.child, fv)
				continue
			}

			// 指定タグを持っていなければスキップ
			if !fp.tagoks[tagidx] {
				continue
			}

			if fp.typ != tt {
				continue
			}

			err := callback(FieldAccess[T]{
				tagidx:          tagidx,
				v:               fv,
				fp:              fp,
				structTag:       fp.sf.Tag, // ★ ここもキャッシュ利用
				selectedTagName: tag,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	return walk(r, root)
}
