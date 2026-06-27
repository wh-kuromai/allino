package allino

import "errors"

//var CreateSessionMap map[string]*GenericFunction[*CreateSessionInput, *CreateSessionOutput, error]

func (rw *GenericFunction[T, U, E]) call_session(r *Runtime, input T, fromcall bool) (output U, err error) {
	r.cache.sessionname = rw.options.Session.Name
	r.cache.sessionversion = rw.options.Session.Version

	var zeroU U
	if fromcall {
		return zeroU, NewError("session can not be used with Handler.Call")
	}

	if rw.options.Session.Type == "sticky" {
		return rw.call_sticky(r, input)
	}

	return zeroU, NewError("invalid settion type")
}

// Called handler's session memory. (not marshaled at any time.)
// *S should be thread-safe.
// SessionInitializer, SessionTimeoutConfirm, SessionFinalizer will be called if S implement it.
func GetSession[S any](r *Runtime) (*S, error) {
	if r.cache.sessiontype == "sticky" {
		entry, err := getStickySessionEntry[S](r)
		if err != nil {
			return nil, err
		}

		val, ok := entry.value.(*S)
		if !ok {
			return nil, NewError("type not match")
		}
		return val, nil
	}

	s, err := getRedisSessionEntryMulti[S](r)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// WithSession aquires mutex.Lock to this session, during mutexFn execution.
func WithSession[S any](r *Runtime, fn func(*S) error) error {
	if r.cache.sessiontype == "sticky" {
		entry, err := getStickySessionEntry[S](r)
		if err != nil {
			return err
		}

		entry.mu.Lock()
		defer entry.mu.Unlock()

		val, ok := entry.value.(*S)
		if !ok {
			return NewError("type not match")
		}

		return fn(val)
	}

	s, err := getRedisSessionEntryMulti[S](r)
	if err != nil {
		return err
	}

	return fn(s)
}

func getRedisSessionEntryMulti[S any](r *Runtime) (*S, error) {
	uid, _, _, err := r.User()
	if errors.Is(ErrNotLoggedIn, err) {
		s, created, err := GetRedisSessionEntry[S](r, r.config.Session.RedisPrefix+":uid:"+uid, r.cache.sessionname)
		if err != nil {
			return nil, err
		}

		if !created {
			return s, nil
		}

		sess_sid, err := getRedisSession(r, r.config.Session.RedisPrefix+":sid:"+r.SessionID(true))
		if err != nil {
			return nil, err
		}

		if sess_sid.created {
			return s, nil
		}

		sess, err := getRedisSession(r, r.config.Session.RedisPrefix+":uid:"+uid)
		if err != nil {
			return nil, err
		}

		if sess != nil {
			for k, v := range sess_sid.value {
				sess.value[k] = v
			}
			sess.checkfns["*"] = func() (bool, error) { return true, nil }
		}

	} else if err != nil {
		return nil, err
	}

	s, _, err := GetRedisSessionEntry[S](r, r.config.Session.RedisPrefix+":sid:"+r.SessionID(true), r.cache.sessionname)
	if err != nil {
		return nil, err
	}

	return s, nil
}
