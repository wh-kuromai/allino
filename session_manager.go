package allino

import (
	"regexp"
	"sync"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
)

type SessionOption struct {
	Type        string
	Name        string
	Version     string
	UseResource map[string]int
}

type SessionConfig struct {
	ServerIDFile  string        `json:"serverid_filename,omitempty"`
	Secret        string        `json:"secret,omitempty"`
	Expire        time.Duration `json:"expire,omitempty"`
	StickeyCookie CookieConfig  `json:"stickey_cookie,omitempty"`

	NodeIP    string `json:"nodeip,omitempty"`
	NodeIPEnv string `json:"nodeip_env,omitempty"`

	ProxyableHosts      []string         `json:"proxyable_hosts,omitempty"`
	ProxyableHostsRegex []*regexp.Regexp `json:"proxyable_hosts_regex,omitempty"`

	Resources map[string]int `json:"resources,omitempty"`

	RedisPrefix string `json:"redis_prefix,omitempty"`

	serverid   string     `json:"-"`
	serveridMu sync.Mutex `json:"-"`
}

func (c *SessionConfig) setup(sv *Server) (*sessionManager, error) {
	s := &sessionManager{
		createSessionMap:   make(map[string]*GenericTypedHandler[*CreateSessionInput, *CreateSessionOutput, error]),
		sessionStore:       make(map[string]*stickySessionEntry),
		resourceConsumeMap: make(map[string]int),
	}

	s.startSessionGC(sv)
	return s, nil
}

var SessionExtension = NewExtension[any, any](
	"session",
	&ExtOption{
		OnHandlerInit: func(s *Server, virtual *Request, opt *HandlerOption) (err error) {
			if opt.Session.Type == "sticky" {
				if s.session == nil {
					s.session, err = s.Config.Session.setup(s)
					if err != nil {
						return err
					}
				}

				s.session.addSessionGroup(s, opt.Session.Name)
			}
			return nil
		},
	},
)

type sessionManager struct {
	// immutable
	createSessionMap map[string]*GenericTypedHandler[*CreateSessionInput, *CreateSessionOutput, error]

	// sessionStore is protected by dequeueMu
	sessionStore map[string]*stickySessionEntry

	// resourceConsumeMap is protected by dequeueMu
	resourceConsumeMap map[string]int

	dequeueMu sync.Mutex
}

type CreateSessionInput struct {
	UseResource map[string]int `json:"use"`
}

type CreateSessionOutput struct {
	Token string `json:"token"`
}

func (s *sessionManager) addSessionGroup(sv *Server, name string) {
	_, ok := s.createSessionMap[name]
	if ok {
		return
	}

	var handler *GenericTypedHandler[*CreateSessionInput, *CreateSessionOutput, error]
	handler = NewTypedHandler(
		HandlerOption{
			Name:    "allino_create_session_" + name,
			Version: "1.0.0",
			JobMode: "dispatch",
			Job: JobOption{
				CacheExpire: 15 * time.Minute,
			},
		},
		func(r *Request, param *CreateSessionInput) (*CreateSessionOutput, error) {
			sid := xid.New().String()

			entry := &stickySessionEntry{
				preserved: true,
				sid:       sid,
				name:      name,
				use:       param.UseResource,
				expireAt:  time.Now().Add(r.config.Session.Expire),
			}

			r.server.session.dequeueMu.Lock()
			defer r.server.session.dequeueMu.Unlock()
			conresult := r.server.session.entry_consume(r, entry)
			if conresult == CONSUME_OVERFLOW {
				r.MarkRequeue()
				return nil, NewError("resource full error")
			}

			r.server.session.sessionStore[entry.sid] = entry

			if conresult == CONSUME_FULL {
				r.server.jobManager.resourcelockedHandlers.Add(encodeHandlerName(handler.options))
			}

			token, err := encodeSession(r, sessionToken{
				NodeIP:    r.server.NodeIP(),
				Name:      name,
				SessionID: sid,
				CreateAt:  time.Now().Unix(),
			})
			if err != nil {
				return nil, err
			}

			if !sv.Config.Log.Silent {
				sv.Logger.Debug("session created", zap.String("name", name))
			}
			return &CreateSessionOutput{
				Token: token,
			}, nil
		},
	)

	s.createSessionMap[name] = handler
	sv.TypedHandle(handler)
}

const (
	CONSUME_OK int = iota
	CONSUME_FULL
	CONSUME_OVERFLOW
)

func (s *sessionManager) entry_consume(r *Request, entry *stickySessionEntry) int {
	if len(entry.use) == 0 {
		return CONSUME_OK
	}

	useResource := entry.use                  // map[string]int  resource to be used.
	resourceConsumed := s.resourceConsumeMap  // map[string]int  count consumed resource
	maxResource := r.config.Session.Resources // map[string]int  max consumable resource

	result := CONSUME_OK

	for k, use := range useResource {
		current := resourceConsumed[k]
		max := maxResource[k]

		// 1回も足せない
		if current+use > max {
			return CONSUME_OVERFLOW
		}

		// 2回足せるかチェック
		if current+use*2 > max {
			// FULL候補（ただし OVERFLOW が優先される）
			result = CONSUME_FULL
		}
	}

	// 実際に消費する（ここで1回分だけ足す）
	for k, use := range useResource {
		resourceConsumed[k] += use
	}

	return result
}

func (s *sessionManager) entry_free(sv *Server, entry *stickySessionEntry) int {
	if len(entry.use) == 0 {
		return CONSUME_OK
	}

	useResource := entry.use
	resourceConsumed := s.resourceConsumeMap

	for k, use := range useResource {
		resourceConsumed[k] -= use
		if resourceConsumed[k] < 0 {
			resourceConsumed[k] = 0 // 念のため保護
		}
	}

	return CONSUME_OK
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
