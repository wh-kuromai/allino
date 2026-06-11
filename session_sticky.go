package allino

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"github.com/wh-kuromai/cryptino"
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
	sid := r.SessionID(true)

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

func (rw *GenericTypedHandler[T, U, E]) call_sticky(r *Request, input T) (output U, err error) {

	var zeroU U
	sessionName := rw.options.Session.Name
	handler := r.server.session.createSessionMap[sessionName]

	c := r.fiber

	ctoken := c.Cookies(r.config.Session.StickeyCookie.Name)

	// check cookie, if not, enqueue CreateSession and Wait.
	if ctoken == "" {
		out, err := handler.Go(r, &CreateSessionInput{UseResource: rw.options.Session.UseResource}).Await()
		if err != nil {
			return zeroU, err
		}

		ctoken = out.Token
	}

	dses, err := decodeSession(r, ctoken)
	if err != nil {
		if errors.Is(err, ErrSessionExpired) {
			out, err := handler.Go(r, &CreateSessionInput{UseResource: rw.options.Session.UseResource}).Await()
			if err != nil {
				return zeroU, err
			}
			ctoken = out.Token

			// ★ 再 decode
			dses, err = decodeSession(r, ctoken)
			if err != nil {
				return zeroU, err
			}
		} else {
			return zeroU, err
		}
	}

	r.cache.sessiontype = "sticky"
	r.cache.sessionid = dses.SessionID
	r.cache.sessionname = dses.Name

	if dses.Name != sessionName {
		return zeroU, NewError("invalid session resource")
	}

	if dses.NodeIP == r.server.nodeip {
		expire := r.config.Session.Expire
		age := time.Now().Unix() - dses.CreateAt

		if expire > 0 && age > int64(expire.Seconds()/2) {
			newToken, err := encodeSession(r, sessionToken{
				NodeIP:    r.server.NodeIP(),
				Name:      sessionName,
				SessionID: dses.SessionID,
				CreateAt:  time.Now().Unix(),
			})
			if err != nil {
				return zeroU, err
			}

			c.Response().Header.SetCookie(
				r.config.Session.StickeyCookie.ToFasthttpCookie(newToken),
			)
		}

		return rw.handlefunc(r, input)
	}

	if r.fiber.Get("X-Allino-Sticky") != "" {
		return zeroU, NewError("fatal: proxy loop")
	}

	if !slices.Contains(r.config.Session.ProxyableHosts, dses.NodeIP) {
		match := false
		for _, re := range r.config.Session.ProxyableHostsRegex {
			if re.MatchString(dses.NodeIP) {
				match = true
				break
			}
		}
		if !match {
			return zeroU, NewError("fatal: proxy failed")
		}
	}

	err = fiberProxy(c, getHostClient(dses.NodeIP))
	if err != nil {
		c.Response().Header.SetCookie(
			r.config.Session.StickeyCookie.ToFasthttpCookie(""),
		)

		return zeroU, NewError("session expired")
	}
	return zeroU, ErrNoOp
}

func fiberProxy(c *fiber.Ctx, hostclient *fasthttp.HostClient) error {
	srcReq := &c.Context().Request

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	srcReq.CopyTo(req)
	req.SetRequestURIBytes(srcReq.URI().RequestURI())

	resp := fasthttp.AcquireResponse()
	// ⚠️ Releaseは defer しない（ストリーム中に使うため）

	// ストリームモード
	err := hostclient.Do(req, resp)
	if err != nil {
		fasthttp.ReleaseResponse(resp)
		return err
	}

	// ヘッダ先にコピー
	resp.Header.CopyTo(&c.Context().Response.Header)
	c.Status(resp.StatusCode())

	// ボディをストリームで流す
	bodyStream := resp.BodyStream()
	if bodyStream != nil {
		//defer bodyStream.Close()

		// Fiber に直接ストリームセット
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			io.Copy(w, bodyStream)
		})
	} else {
		// fallback（小さいレスポンス）
		c.Context().Write(resp.Body())
	}

	return nil
}

func (s *Server) NodeIP() string {
	if s.nodeip != "" {
		return s.nodeip
	}

	if s.Config.Session.NodeIP != "" {
		return s.Config.Session.NodeIP
	}

	if s.Config.Session.NodeIPEnv != "" {
		val, ok := s.Env[s.Config.Session.NodeIPEnv]
		if ok {
			s.nodeip = val
			return val
		}
	}

	host, port, err := net.SplitHostPort(s.Config.Bind)
	if err != nil {
		return ""
	}

	if host == "" || host == "0.0.0.0" {
		host = getLocalIP()
		if host == "" {
			host = "localhost" // fallback
		}
	}

	val := net.JoinHostPort(host, port)
	s.nodeip = val
	return val
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		// down or loopback は除外
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			if ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String()
		}
	}

	return ""
}

type sessionToken struct {
	NodeIP    string `json:"n"`
	Name      string `json:"r"`
	SessionID string `json:"s"`
	CreateAt  int64  `json:"c,omitempty"`
}

func encodeSession(r *Request, data sessionToken) (string, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(r.config.Session.Secret))
	ebuf, err := cryptino.EncryptByGCM(key[:], buf)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(ebuf), nil
}

var ErrSessionExpired = NewError("session expired")

func decodeSession(r *Request, token string) (*sessionToken, error) {
	key := sha256.Sum256([]byte(r.config.Session.Secret))
	ebuf, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	buf, err := cryptino.DecryptByGCM(key[:], ebuf)
	if err != nil {
		return nil, err
	}
	// decrypt and decode session
	var data sessionToken
	err = json.Unmarshal(buf, &data)
	if err != nil {
		return nil, err
	}

	if r.config.Session.Expire > 0 {
		if time.Now().After(time.Unix(data.CreateAt, 0).Add(r.config.Session.Expire)) {
			return nil, ErrSessionExpired
		}
	}

	return &data, nil
}
