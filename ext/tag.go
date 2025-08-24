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
	sliceMatch      bool
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

	// Handle nil values explicitly
	val := reflect.ValueOf(value)
	if !val.IsValid() {
		fv.Set(reflect.Zero(fv.Type()))
		return
	}

	// Ensure assignability: value's dynamic type must be assignable to the field
	if val.Type().AssignableTo(fv.Type()) {
		fv.Set(val)
		return
	}
	// Also allow conversion if possible
	if val.Type().ConvertibleTo(fv.Type()) {
		fv.Set(val.Convert(fv.Type()))
		return
	}
	// Otherwise, do nothing (or panic/log, depending on policy)
}

// SetFillSlice builds a slice value for this field and sets it.
//
// Behavior:
//   - The underlying field must be a slice; otherwise this is a no-op.
//   - For each i in [0,size), a prototype value v of the element type is created and
//     passed to `fill(i, v)`.
//   - If the element type is a pointer, v is a new instance of *Elem (allocated via reflect.New(Elem)).
//   - If the element type is a non-pointer, v is the zero value of Elem.
//   - If fill returns (vv, true), vv is written into the slice at index i.
//     Assignable / Convertible values are accepted. If the element type is a pointer
//     and vv is a non-pointer Elem, a new pointer is allocated and populated.
//   - Elements where ok==false remain the zero value.
func (f *FieldAccess[T]) SetFillSlice(size int, fill func(i int, v any) (vv any, ok bool)) {
	if f == nil {
		return
	}
	fv := f.v
	if !fv.IsValid() || !fv.CanSet() {
		return
	}
	if size < 0 {
		size = 0
	}
	// Only works on slice fields
	if fv.Kind() != reflect.Slice {
		return
	}

	sliceType := fv.Type()
	elemType := sliceType.Elem()
	sliceVal := reflect.MakeSlice(sliceType, size, size)

	for i := 0; i < size; i++ {
		// Prepare prototype v for filler
		var v any
		if elemType.Kind() == reflect.Ptr {
			v = reflect.New(elemType.Elem()).Interface() // *Elem
		} else {
			v = reflect.Zero(elemType).Interface() // Elem (zero)
		}

		if fill == nil {
			continue
		}
		vv, ok := fill(i, v)
		if !ok {
			continue
		}

		val := reflect.ValueOf(vv)
		// Direct assignable
		if val.IsValid() && val.Type().AssignableTo(elemType) {
			sliceVal.Index(i).Set(val)
			continue
		}
		// Convertible
		if val.IsValid() && val.Type().ConvertibleTo(elemType) {
			sliceVal.Index(i).Set(val.Convert(elemType))
			continue
		}
		// If element is a pointer and vv is non-pointer Elem, box it
		if elemType.Kind() == reflect.Ptr && val.IsValid() {
			if val.Type().AssignableTo(elemType.Elem()) {
				p := reflect.New(elemType.Elem())
				p.Elem().Set(val)
				sliceVal.Index(i).Set(p)
				continue
			}
			if val.Type().ConvertibleTo(elemType.Elem()) {
				p := reflect.New(elemType.Elem())
				p.Elem().Set(val.Convert(elemType.Elem()))
				sliceVal.Index(i).Set(p)
				continue
			}
		}
		// Otherwise leave zero value
	}

	fv.Set(sliceVal)
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

	if !f.sliceMatch {
		if fv.Kind() == reflect.Ptr && fv.IsNil() {
			return zero
		}
		return fv.Interface().(T)
	}

	// sliceMatch モード
	if fv.Kind() != reflect.Slice {
		return zero
	}

	// T が slice であることを確認
	tt := reflect.TypeOf(zero)
	if tt.Kind() != reflect.Slice {
		return zero
	}

	elemT := tt.Elem() // T の要素型（例: *MyStruct）
	n := fv.Len()
	out := reflect.MakeSlice(tt, n, n)

	for i := 0; i < n; i++ {
		elem := fv.Index(i)
		if !elem.IsValid() {
			continue
		}
		if !elem.CanInterface() {
			continue
		}

		val := reflect.ValueOf(elem.Interface())

		// そのまま代入可能
		if val.Type().AssignableTo(elemT) {
			out.Index(i).Set(val)
			continue
		}

		// 変換可能
		if val.Type().ConvertibleTo(elemT) {
			out.Index(i).Set(val.Convert(elemT))
			continue
		}

		// ポインタ詰め替え（*ElemT の場合）
		if elemT.Kind() == reflect.Ptr && val.Type().AssignableTo(elemT.Elem()) {
			p := reflect.New(elemT.Elem())
			p.Elem().Set(val)
			out.Index(i).Set(p)
			continue
		}
	}

	return out.Interface().(T)
}

// FieldPlan provides a metadata-only handle to a field registered in a ReflectPlan.
// It mirrors FieldAccess but does not carry a concrete reflect.Value, so it can be
// used without a target instance. You can later obtain a FieldAccess via Accessor().
type FieldPlan[T any] struct {
	tagidx          int
	fp              *fieldPlan
	structTag       reflect.StructTag
	selectedTagName string
}

func (f *FieldPlan[T]) Name() string {
	if f == nil || f.fp == nil || f.tagidx < 0 || f.tagidx >= len(f.fp.tags) {
		return ""
	}
	return f.fp.name
}

func (f *FieldPlan[T]) Tag() string {
	if f == nil || f.fp == nil || f.tagidx < 0 || f.tagidx >= len(f.fp.tags) {
		return ""
	}
	return f.fp.tags[f.tagidx]
}

func (f *FieldPlan[T]) Lookup(tag string) (string, bool) {
	if f == nil {
		return "", false
	}
	return f.structTag.Lookup(tag)
}

func (f *FieldPlan[T]) GetLocal() any {
	if f == nil {
		return nil
	}
	return f.fp.local
}
func (f *FieldPlan[T]) SetLocal(v any) {
	if f != nil {
		f.fp.local = v
	}
}

// Type information helpers
func (f *FieldPlan[T]) Type() reflect.Type {
	if f == nil || f.fp == nil {
		return nil
	}
	return f.fp.typ
}
func (f *FieldPlan[T]) BaseType() reflect.Type {
	if f == nil || f.fp == nil {
		return nil
	}
	return f.fp.basetyp
}
func (f *FieldPlan[T]) Kind() reflect.Kind {
	if f == nil || f.fp == nil {
		return reflect.Invalid
	}
	return f.fp.kind
}
func (f *FieldPlan[T]) IsPointer() bool {
	if f == nil || f.fp == nil {
		return false
	}
	return f.fp.ispointer
}
func (f *FieldPlan[T]) Index() []int {
	if f == nil || f.fp == nil {
		return nil
	}
	return slices.Clone(f.fp.index)
}

// Accessor builds a FieldAccess for a concrete target using the stored index path.
// Returns (zero, false) if the target is incompatible.
func (f *FieldPlan[T]) Accessor(target any) (FieldAccess[T], bool) {
	var zero FieldAccess[T]
	if f == nil || f.fp == nil || target == nil {
		return zero, false
	}
	root := reflect.ValueOf(target)
	if root.Kind() == reflect.Ptr {
		root = root.Elem()
	}
	if !root.IsValid() || root.Kind() != reflect.Struct {
		return zero, false
	}

	// type check consistency with T
	tt := reflect.TypeOf((*T)(nil)).Elem()
	if !typeMatch(tt, f.fp.typ) {
		return zero, false
	}

	fv := root.FieldByIndex(f.fp.index)
	if !fv.IsValid() {
		return zero, false
	}

	return FieldAccess[T]{
		tagidx:          f.tagidx,
		v:               fv,
		fp:              f.fp,
		structTag:       f.fp.sf.Tag,
		selectedTagName: f.selectedTagName,
	}, true
}

var (
	ErrNotStruct       = errors.New("not struct or struct pointer")
	ErrNotRegisterdTag = errors.New("not pre-registered tag")
)

// typeMatches returns true if the field type exactly equals T, or
// if T is an interface that the field type (or its pointer) implements.
func typeMatch(tt, fieldType reflect.Type) bool {
	if tt == nil || fieldType == nil {
		return false
	}
	if fieldType == tt {
		return true
	}
	if tt.Kind() == reflect.Interface {
		// Direct implementation by the field type
		if fieldType.Implements(tt) {
			return true
		}
		// If the field is a non-pointer, check pointer receiver methods too
		if fieldType.Kind() != reflect.Ptr {
			if reflect.PointerTo(fieldType).Implements(tt) {
				return true
			}
		}
	}
	return false
}

// typeSliceMatch returns true if both tt and fieldType are slices, and their
// element types match according to typeMatches.
func typeSliceMatch(tt, fieldType reflect.Type) bool {
	if tt == nil || fieldType == nil {
		return false
	}
	if tt.Kind() != reflect.Slice || fieldType.Kind() != reflect.Slice {
		return false
	}

	elemT := tt.Elem()
	elemF := fieldType.Elem()

	// allow exact element type match
	if elemT == elemF {
		return true
	}

	// allow interface compatibility
	return typeMatch(elemT, elemF)
}

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

			sliceMatch := typeSliceMatch(tt, fp.typ)
			if !sliceMatch && !typeMatch(tt, fp.typ) {
				continue
			}

			err := callback(FieldAccess[T]{
				tagidx:          tagidx,
				v:               fv,
				fp:              fp,
				structTag:       fp.sf.Tag, // ★ ここもキャッシュ利用
				selectedTagName: tag,
				sliceMatch:      sliceMatch,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	return walk(r, root)
}

// EachPlan walks a ReflectPlan without requiring a concrete target instance.
// It calls the callback for each field that both (a) has the specified `tag` and
// (b) has the exact type T. Nested non-pointer structs are traversed recursively
// following the pre-built plan (same behavior as ExecutePlan).
func EachPlan[T any](r *ReflectPlan, tag string, callback func(f FieldPlan[T]) error) error {
	if r == nil || callback == nil || tag == "" {
		return ErrNotStruct
	}

	tagidx := slices.Index(r.taglist, tag)
	if tagidx < 0 {
		return ErrNotRegisterdTag
	}

	tt := reflect.TypeOf((*T)(nil)).Elem()

	var walk func(rp *ReflectPlan) error
	walk = func(rp *ReflectPlan) error {
		if rp == nil {
			return ErrNotStruct
		}
		for _, fp := range rp.fields {
			// Recurse into nested non-pointer structs captured in the plan
			if fp.child != nil {
				if err := walk(fp.child); err != nil {
					return err
				}
			}

			// filter: tag existence
			if !fp.tagoks[tagidx] {
				continue
			}

			// filter: type match with T
			if !typeSliceMatch(tt, fp.typ) && !typeMatch(tt, fp.typ) {
				continue
			}

			if err := callback(FieldPlan[T]{
				tagidx:          tagidx,
				fp:              fp,
				structTag:       fp.sf.Tag,
				selectedTagName: tag,
			}); err != nil {
				return err
			}
		}
		return nil
	}

	return walk(r)
}
