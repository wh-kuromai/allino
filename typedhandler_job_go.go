package allino

import (
	"sync"
)

type Future[U any] struct {
	done chan struct{}

	mu     sync.Mutex
	result U
	err    error
	doneOk bool

	thenFns    []func(U)
	catchFns   []func(error)
	finallyFns []func()
}

func NewFuture[U any]() *Future[U] {
	return &Future[U]{
		done: make(chan struct{}),
	}
}

func (f *Future[U]) returns(output U, err error) {
	f.mu.Lock()
	if f.doneOk {
		f.mu.Unlock()
		return
	}

	f.result = output
	f.err = err
	f.doneOk = true

	thenFns := f.thenFns
	catchFns := f.catchFns
	finallyFns := f.finallyFns

	f.mu.Unlock()

	// close first (waiters unblock)
	close(f.done)

	// callbacks
	if err == nil {
		for _, fn := range thenFns {
			fn(output)
		}
	} else {
		for _, fn := range catchFns {
			fn(err)
		}
	}

	for _, fn := range finallyFns {
		fn()
	}
}

func (f *Future[U]) Then(fn func(U)) *Future[U] {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.doneOk && f.err == nil {
		// already done → 即実行
		go fn(f.result)
		return f
	}

	f.thenFns = append(f.thenFns, fn)
	return f
}

func (f *Future[U]) Catch(fn func(error)) *Future[U] {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.doneOk && f.err != nil {
		go fn(f.err)
		return f
	}

	f.catchFns = append(f.catchFns, fn)
	return f
}

func (f *Future[U]) Finally(fn func()) *Future[U] {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.doneOk {
		go fn()
		return f
	}

	f.finallyFns = append(f.finallyFns, fn)
	return f
}

// optional: 同期的に待つ
func (f *Future[U]) Await() (U, error) {
	<-f.done
	return f.result, f.err
}

func (rw *GenericTypedHandler[T, U, E]) Go(r *Request, input T) *Future[U] {
	//f := &Future[U]{}
	f := NewFuture[U]()
	rw.go_internal(r, input, f)
	return f
}

func (rw *GenericTypedHandler[T, U, E]) go_internal(r *Request, input T, f *Future[U]) {
	out, err := rw.Call(r, input)

	waitf := func(perr *JobPendingError) {
		var zeroU U
		c := rw.options.Job.callstrategy
		_, _, _, syserr := c.Wait(r.Context(), perr.JobID, rw.options.Job.CacheExpire != 0, r.server.TimeWheel)
		if syserr != nil {
			f.returns(zeroU, syserr)
			return
		}
		rw.go_internal(r, input, f)
	}

	switch perr := err.(type) {
	case *JobPendingError:
		go waitf(perr)
		return
	case interface{ Unwrap() error }:
		perrw := perr.Unwrap()
		if perrw != nil {
			pperr, ok := perrw.(*JobPendingError)
			if ok {
				go waitf(pperr)
				return
			}
		}
	case interface{ Unwrap() []error }:
		for _, perrw := range perr.Unwrap() {
			if perrw != nil {
				pperr, ok := perrw.(*JobPendingError)
				if ok {
					go waitf(pperr)
					return
				}
			}
		}
	}

	f.returns(out, err)
}
