package allino

import (
	"reflect"
)

type tagKind uint8

const (
	tagPath tagKind = iota
	tagQuery
	tagForm
	tagPost
	tagJWT
	tagCookie
	tagHeader
	tagRegex
	tagSize
)

var tagKindStrings = []string{
	"path",
	"query",
	"form",
	"post",
	"jwt",
	"cookie",
	"header",
	"regex",
}

func (p tagKind) String() string {
	return tagKindStrings[p]
}

type fieldPlan struct {
	tags      []string // len(tags) == tagSize, fieldPlan[tagKind] = field.Tag.LookUp(tagKind.String())
	tagoks    []bool
	name      string
	typ       reflect.Type
	basetyp   reflect.Type
	kind      reflect.Kind
	ispointer bool

	// 非ポインタの struct フィールド用に、ネストを再帰的に解析した結果を保持
	child *reflectPlan
}

type reflectPlan struct {
	typ    reflect.Type
	fields []*fieldPlan
}

func buildPlan(t reflect.Type) *reflectPlan {
	// ポインタなら剥がす
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		// 想定外の型でも落ちないように空プランを返す
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
			tags:   make([]string, tagSize),
			tagoks: make([]bool, tagSize),
			name:   sf.Name,
			typ:    sf.Type,
			kind:   sf.Type.Kind(),
		}

		// ポインタ型の場合は element type を扱う
		if sf.Type.Kind() == reflect.Ptr {
			fp.ispointer = true
			fp.basetyp = sf.Type.Elem()
		} else {
			fp.basetyp = sf.Type
		}

		// すべてのタグを一度に前計算
		for k := tagKind(0); k < tagSize; k++ {
			tagName := k.String()
			if v, ok := sf.Tag.Lookup(tagName); ok {
				if v == "" {
					fp.tags[k] = sf.Name
				} else {
					fp.tags[k] = v
				}
				fp.tagoks[k] = true
			}
		}

		// ★ ここがポイント：非ポインタの struct フィールドだけ再帰
		if fp.basetyp.Kind() == reflect.Struct && !fp.ispointer {
			// 匿名かどうかに関係なく、子プランとして保持（フラット化はしない）
			fp.child = buildPlan(fp.basetyp)
		}

		fields = append(fields, fp)

		// --- 将来の拡張メモ ---
		// ・匿名埋め込み struct をフラット化したい場合は、
		//   if sf.Anonymous && sf.Type.Kind()==reflect.Struct { 再帰して展開 }
		//   ただしその際は fieldPlan に index []int（FieldByIndex 用）を追加すること。
		// ・post:"json/xml/raw" のような挙動フラグ（setDirect 等）は、この段階で
		//   解析して fieldPlan に boolean を持たせると実行時の分岐が軽くなる。
		// ・validator 用の required / default / fallback なども同様に前処理しておくと良い。
	}

	return &reflectPlan{
		typ:    t,
		fields: fields,
	}
}
