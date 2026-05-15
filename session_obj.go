package allino

import (
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
)

//var sessionStore sync.Map // map[string]*sessionEntry

type SessionInitializer interface {
	New(r *Server) any
}

type SessionKeepAliver interface {
	ShouldKeepAlive(s *Server) bool
}

type SessionFinalizer interface {
	Close(s *Server) error
}

// Called handler's session memory. (not marshaled at any time.)
// *S should be thread-safe.
// SessionInitializer, SessionTimeoutConfirm, SessionFinalizer will be called if S implement it.
func GetSession[S any](r *Request) (*S, error) {
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

	entry, err := getRedisSessionEntry[S](r)
	if err != nil {
		return nil, err
	}

	return entry.value, nil
}

// WithSession aquires mutex.Lock to this session, during mutexFn execution.
func WithSession[S any](r *Request, fn func(*S) error) error {
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

	entry, err := getRedisSessionEntry[S](r)
	if err != nil {
		return err
	}

	return fn(entry.value)
}

type stickySessionEntry struct {
	preserved bool
	sid       string
	name      string
	use       map[string]int
	value     any
	expireAt  time.Time
	mu        sync.Mutex
}

func getStickySessionEntry[S any](r *Request) (*stickySessionEntry, error) {
	sid := r.SessionID()

	now := time.Now()

	r.server.session.dequeueMu.Lock()
	defer r.server.session.dequeueMu.Unlock()

	entry, ok := r.server.session.sessionStore[sid]
	if ok {
		if entry.value != nil {
			entry.expireAt = now.Add(r.config.Session.Expire)
			return entry, nil
		}

		// create new
		var instance *S

		// initializer対応
		var tmp S
		if init, ok := any(&tmp).(SessionInitializer); ok {
			v := init.New(r.server)
			instance, ok = v.(*S)
			if !ok {
				return nil, NewError("type not match")
			}
		} else {
			instance = new(S)
		}

		entry.value = instance
		entry.expireAt = now.Add(r.config.Session.Expire)
		return entry, nil
	}

	return nil, NewError("session not exist")
}

type redisSessionEntry[S any] struct {
	value   *S
	version string
	deferfn func() error
}

func getRedisSessionEntry[S any](r *Request) (*redisSessionEntry[S], error) {
	result, err := r.Redis().MGet(r.Context(),
		r.config.Session.RedisPrefix+":"+r.cache.sessionname+":value:"+r.SessionID(),
		r.config.Session.RedisPrefix+":"+r.cache.sessionname+":version:"+r.SessionID(),
	).Result()

	if err != nil {
		return nil, err
	}

	if len(result) != 2 {
		return nil, FatalInvalidCacheError
	}

	sess := &redisSessionEntry[S]{}

	version, ok := result[1].(string)
	if ok {
		sess.version = version
	}

	value, ok := result[0].(string)
	if !ok {
		return nil, FatalInvalidCacheError
	}

	sess.deferfn = func() error {
		mbuf, err := json.Marshal(sess.value)
		if err != nil {
			return err
		}

		if value == string(mbuf) {
			return nil
		}

		err = r.Redis().Set(r.Context(),
			r.config.Session.RedisPrefix+":"+r.cache.sessionname+":value:"+r.SessionID(),
			string(mbuf), r.config.Session.Expire).Err()
		if err != nil {
			return err
		}

		err = r.Redis().Set(r.Context(),
			r.config.Session.RedisPrefix+":"+r.cache.sessionname+":version:"+r.SessionID(),
			r.cache.sessionversion, r.config.Session.Expire).Err()

		return err
	}

	r.Defer(sess.deferfn)

	if hasMajorOrMinorVersionDiff(r.cache.sessionversion, sess.version) {
		tfn := getSessionUpgrader[S](r.cache.sessionname)
		if tfn != nil {
			tres, err := tfn(sess.version, []byte(value))
			if tres != nil {
				if err != nil {
					return nil, err
				}
				sess.value = tres
				return sess, nil
			}
		}
	}

	var sval S
	err = json.Unmarshal([]byte(value), &sval)
	if err != nil {
		return nil, err
	}

	sess.value = &sval
	return sess, nil
}

var sessionUpgraderMap sync.Map

func RegisterSessionUpgrader[S any](sessionname string, fn func(version string, olddata []byte) (*S, error)) error {
	sessionUpgraderMap.Store(sessionname, fn)
	return nil
}

func getSessionUpgrader[S any](sessionname string) func(version string, olddata []byte) (*S, error) {
	fn, ok := sessionUpgraderMap.Load(sessionname)
	if ok {
		ffn, ok := fn.(func(version string, olddata []byte) (*S, error))
		if ok {
			return ffn
		}
	}

	return nil
}

func (s *sessionManager) startSessionGC(sv *Server) {
	sv.TimeWheel.Add(time.Minute, func() bool {
		now := time.Now()

		s.dequeueMu.Lock()
		defer s.dequeueMu.Unlock()

		for k, entry := range s.sessionStore {
			if now.After(entry.expireAt) {
				s.mayDeleteSession(sv, k, entry)
			}
		}
		return true
	})
}

func (s *sessionManager) mayDeleteSession(sv *Server, sid string, entry *stickySessionEntry) bool {
	if f, ok := entry.value.(SessionKeepAliver); ok {
		ka := f.ShouldKeepAlive(sv)
		if ka {
			entry.expireAt = time.Now().Add(sv.Config.Session.Expire)
			return false
		}
	}

	if f, ok := entry.value.(SessionFinalizer); ok {
		err := f.Close(sv)
		if err != nil && !sv.Config.Log.Silent {
			sv.Logger.Error("session close failed", zap.Error(err))
		}
	}

	delete(s.sessionStore, sid)

	conresult := s.entry_free(sv, entry)
	if conresult == CONSUME_OK {
		handler := encodeHandlerName(s.createSessionMap[entry.name].options)
		sv.jobManager.resourcelockedHandlers.Remove(handler)
	}

	return true
}
