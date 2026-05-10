package allino

import (
	"encoding/json"
	"reflect"
	"sync"
)

type ReflectPool[T any] struct {
	ttype     reflect.Type
	isany     bool
	ispointer bool
	pool      sync.Pool

	newfn func() any
}

func NewReflectPool[T any]() *ReflectPool[T] {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	if tType.Kind() == reflect.Interface && tType.NumMethod() == 0 {
		//panic("reflect pool does not accept any")
		return &ReflectPool[T]{isany: true}
	}
	if tType.Kind() == reflect.Ptr {
		nfn := func() any {
			return reflect.New(tType.Elem()).Interface()
		}
		return &ReflectPool[T]{
			ispointer: true,
			ttype:     tType,
			pool: sync.Pool{
				New: nfn,
			},
			newfn: nfn,
		}
	}
	return &ReflectPool[T]{ttype: tType}
}

func (r *ReflectPool[T]) New(fn func(a any) error) (newt T, err error) {
	if r.isany {
		return newt, nil
	}

	if r.ispointer {
		newTt := r.newfn().(T)
		err = fn(newTt)
		return newTt, err
	}

	err = fn(&newt)
	return
}

func (r *ReflectPool[T]) Acquire(fn func(a any) error) (newt T, err error) {
	if r.isany {
		return newt, nil
	}

	if r.ispointer {
		newTt := r.pool.Get().(T)
		err = fn(newTt)
		return newTt, err
	}

	err = fn(&newt)
	return
}

func (r *ReflectPool[T]) AcquireUnmarshalJSON(buf []byte) (newt T, err error) {
	return r.Acquire(func(a any) error {
		return json.Unmarshal(buf, a)
	})
}

func (r *ReflectPool[T]) Release(oldt T) {
	if r.ispointer {
		r.pool.Put(oldt)
	}
}
