package allino

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/wh-kuromai/cryptino"
	"go.uber.org/zap"
)

type LoginConfig struct {
	//RawPublicKey  RawMessage      `json:"publickey"`
	//RawPrivateKey RawMessage      `json:"privatekey"`
	OAuth       OAuthConfig     `json:"oauth"`
	CSRFToken   CSRFTokenConfig `json:"csrf"`
	LoginCookie CookieConfig    `json:"cookie"`
	GuestCookie CookieConfig    `json:"guest_cookie"`

	PublicKey  cryptino.PublicKey  `json:"publickey"`
	PrivateKey cryptino.PrivateKey `json:"privatekey"`
}

type CSRFTokenConfig struct {
	Expire      time.Duration `json:"expire"`
	QueryKey    string        `json:"querykey"`
	FormKey     string        `json:"formkey"`
	JWTAudience string        `json:"jwt_audience"`
}

type OAuthConfig struct {
	Expire      time.Duration `json:"expire"`
	ExpireLong  time.Duration `json:"expire_longterm"`
	QueryKey    string        `json:"querykey"`
	FormKey     string        `json:"formkey"`
	AuthBearer  bool          `json:"authbearer"`
	JWTAudience string        `json:"jwt_audience"`
}

type CookieConfig struct {
	Name        string        `json:"name"`
	Expire      time.Duration `json:"expire"`
	Secure      bool          `json:"secure"`
	Httponly    bool          `json:"httponly"`
	SameSite    string        `json:"samesite"`
	Path        string        `json:"path"`
	JWTAudience string        `json:"jwt_audience"`
}

var (
	ErrNoPublicKey = errors.New("Config login.publickey or login.privatekey has not valid key")
	ErrNotLoggedIn = errors.New("User not logged-in")
)

func (c *LoginConfig) setup() error {
	if c.PrivateKey != nil {
		c.PublicKey = c.PrivateKey.Public()
	}
	return nil
}

/*
func (c *LoginConfig) PublicKey() cryptino.PublicKey {
	if c.pubKey != nil {
		return c.pubKey
	}

	pk := c.PrivateKey()
	if pk != nil {
		return pk.Public()
	}

	if c.RawPublicKey != nil {
		pub, err := cryptino.UnmarshalJSONPublicKey(c.RawPublicKey)
		if err == nil {
			c.pubKey = pub
		}
		return pub
	}

	return nil
}

func (c *LoginConfig) PrivateKey() cryptino.PrivateKey {
	if c.privKey != nil {
		return c.privKey
	}

	if c.RawPrivateKey != nil {
		pk, err := cryptino.UnmarshalJSONPrivateKey(c.RawPrivateKey)
		if err == nil {
			c.privKey = pk
		}
		return pk
	}
	return nil
}
*/

func (r *Request) User() (uid, displayname string, writable bool, err error) {
	uid, displayname, writable, _, err = r.userWithJWT()
	return
}

func (r *Request) Claim(key string) (value string, err error) {
	_, _, _, jwtbody, err := r.userWithJWT()
	if err != nil {
		return "", err
	}

	claims, err := r.jwtdecodedbyclaims(jwtbody)
	if err != nil {
		return "", err
	}

	value, ok := claims[key]
	if !ok {
		return "", errors.New("claim not found")
	}

	return
}

func (r *Request) ClaimUnmarshal(obj any) error {
	_, _, _, jwtbody, err := r.userWithJWT()
	if err != nil {
		return err
	}

	return json.Unmarshal(jwtbody, obj)
}

func (r *Request) userWithJWT() (uid, displayname string, writable bool, jwtbody []byte, err error) {
	if r.cache.cachedLogin {
		return r.cache.cachedUid, r.cache.cachedName, r.cache.cachedWritable, r.cache.cachedJWTBody, r.cache.cachedErr
	}

	r.cache.cachedUid, r.cache.cachedName, r.cache.cachedWritable, r.cache.cachedJWTBody, r.cache.cachedErr = r.user()
	r.cache.cachedLogin = true
	return r.cache.cachedUid, r.cache.cachedName, r.cache.cachedWritable, r.cache.cachedJWTBody, r.cache.cachedErr
}

func (r *Request) user() (uid, displayname string, writable bool, body []byte, err error) {
	du := r.fiber.Query(".user")
	if r.config.Debug && du != "" {
		r.cache.authorizedBy = "debug"
		return du, du, true, nil, nil
	}

	pub := r.config.Login.PublicKey
	if pub == nil {
		return "", "", false, nil, ErrNoPublicKey
	}

	accessToken := ""
	if r.config.Login.OAuth.QueryKey != "" {
		accessToken = r.fiber.Query(r.config.Login.OAuth.QueryKey)
	}
	if accessToken == "" && r.config.Login.OAuth.FormKey != "" {
		accessToken = r.fiber.FormValue(r.config.Login.OAuth.FormKey)
	}

	if accessToken == "" && r.config.Login.OAuth.AuthBearer {
		authheader := r.fiber.Get("Authorization")
		if strings.HasPrefix(authheader, "Bearer ") {
			accessToken = strings.TrimSpace(authheader[len("Bearer "):])
		}
	}

	if accessToken != "" {
		oauthjwt, err := cryptino.VerifyJWT(cryptino.ES256(), []byte(accessToken), pub)

		if err != nil {
			r.Logger().Debug("AccessToken Verify Error", zap.Error(err))
		} else if oauthjwt.Body.Audience == r.config.Login.OAuth.JWTAudience {
			r.cache.authorizedBy = r.config.Login.OAuth.JWTAudience
			return oauthjwt.Body.Subject, oauthjwt.Body.Name, true, oauthjwt.RawBody, nil
		}
	}

	cookie := r.fiber.Cookies(r.config.Login.LoginCookie.Name)

	if cookie != "" {
		cookiejwt, err := cryptino.VerifyJWT(cryptino.ES256(), []byte(cookie), pub)
		if err != nil {
			r.Logger().Debug("Cookie Verify Error", zap.Error(err))
		} else if cookiejwt.Body.Audience == r.config.Login.LoginCookie.JWTAudience {

			csrftoken := r.fiber.Get("X-CSRF-Token")
			if csrftoken == "" && r.config.Login.CSRFToken.FormKey != "" {
				csrftoken = r.fiber.FormValue(r.config.Login.CSRFToken.FormKey)
			}
			if csrftoken == "" && r.config.Login.CSRFToken.QueryKey != "" {
				csrftoken = r.fiber.Query(r.config.Login.CSRFToken.QueryKey)
			}

			if csrftoken != "" {
				csrfjwt, err := cryptino.VerifyJWT(cryptino.ES256(), []byte(csrftoken), pub)
				if err != nil {
					//r.Logger.Debug("CSRFToken Verify Error", zap.Error(err))
				} else if csrfjwt.Body.Audience == r.config.Login.CSRFToken.JWTAudience && cookiejwt.Body.Subject == csrfjwt.Body.Subject {
					r.cache.authorizedBy = r.config.Login.CSRFToken.JWTAudience
					return cookiejwt.Body.Subject, cookiejwt.Body.Name, true, cookiejwt.RawBody, nil
				}
			}

			r.cache.authorizedBy = r.config.Login.LoginCookie.JWTAudience
			return cookiejwt.Body.Subject, cookiejwt.Body.Name, false, cookiejwt.RawBody, nil
		}
	}

	return "", "", false, nil, ErrNotLoggedIn
}

func (r *Request) SessionID() string {
	if r.cache.sessionid != "" {
		return r.cache.sessionid
	}
	guestcookie := r.fiber.Cookies(r.config.Login.GuestCookie.Name)
	//r.Log.Print(loghub.TRACE, "cookie:", cookie, err)

	if guestcookie != "" {
		pub := r.config.Login.PublicKey
		if pub != nil {
			cookiejwt, err := cryptino.VerifyJWT(cryptino.ES256(), []byte(guestcookie), pub)
			if err == nil {
				if cookiejwt.Body.Audience == r.config.Login.GuestCookie.JWTAudience {
					r.cache.sessionid = cookiejwt.Body.Subject
					r.cache.guestcookiefound = true
				}
			}
		}
	}

	if r.cache.sessionid == "" {
		r.cache.sessionid = uuid.New().String() // fallback to new session id
	}

	if !r.issubrequest {
		ngcookie := IssueGuestCookie(r)
		r.fiber.Cookie(ngcookie)
	}

	return r.cache.sessionid
}

func (r *Request) AssumeUser(uid, displayname string, writable bool) {
	r.cache.cachedLogin = true
	r.cache.cachedUid = uid
	r.cache.cachedName = displayname
	r.cache.cachedWritable = writable
	r.cache.authorizedBy = "simulate"
}

func (r *Request) AuthorizedBy() string {
	return r.cache.authorizedBy
}

func IssueCSRFToken(r *Request, uid string) string {
	jwt := cryptino.GetJWTBasic(uid, r.config.Login.CSRFToken.Expire)
	jwt.Body.Audience = r.config.Login.CSRFToken.JWTAudience

	csrftoken, err := jwt.Marshal(cryptino.ES256(), r.config.Login.PrivateKey)
	if err != nil {
		r.Logger().Error("IssueCSRFToken: config.login.privatekey is not valid", zap.Error(err))
		return ""
	}
	return csrftoken
}

func IssueAccessToken(r *Request, uid, name string, custom ...map[string]any) string {
	jwt := cryptino.GetJWTBasic(uid, r.config.Login.OAuth.Expire)
	jwt.Body.Audience = r.config.Login.OAuth.JWTAudience
	jwt.Body.Name = name
	if len(custom) > 0 {
		jwt.Body.Custom = custom[0]
	}

	accesstoken, err := jwt.Marshal(cryptino.ES256(), r.config.Login.PrivateKey)
	if err != nil {
		r.Logger().Error("IssueAccessToken: config.login.privatekey is not valid", zap.Error(err))
		return ""
	}
	return accesstoken
}

func IssueAPIKey(r *Request, uid, name string, custom ...map[string]any) string {
	jwt := cryptino.GetJWTBasic(uid, r.config.Login.OAuth.ExpireLong)
	jwt.Body.Audience = r.config.Login.OAuth.JWTAudience
	jwt.Body.Name = name
	if len(custom) > 0 {
		jwt.Body.Custom = custom[0]
	}

	accesstoken, err := jwt.Marshal(cryptino.ES256(), r.config.Login.PrivateKey)
	if err != nil {
		r.Logger().Error("IssueAPIKey: config.login.privatekey is not valid", zap.Error(err))
		return ""
	}
	return accesstoken
}

func IssueLoginCookie(r *Request, uid, name string, custom ...map[string]any) *fiber.Cookie {
	jwt := cryptino.GetJWTBasic(uid, r.config.Login.LoginCookie.Expire)
	jwt.Body.Audience = r.config.Login.LoginCookie.JWTAudience
	jwt.Body.Name = name
	if len(custom) > 0 {
		jwt.Body.Custom = custom[0]
	}

	jwtbuf, err := jwt.Marshal(cryptino.ES256(), r.config.Login.PrivateKey)
	if err != nil {
		r.Logger().Error("IssueLoginCookie: config.login.privatekey is not valid", zap.Error(err))
		return nil
	}

	// set cookie for storing token
	cookie := &fiber.Cookie{
		Name:     r.config.Login.LoginCookie.Name,
		Value:    string(jwtbuf),
		Expires:  time.Now().Add(r.config.Login.LoginCookie.Expire * time.Second),
		Secure:   r.config.Login.LoginCookie.Secure,
		SameSite: r.config.Login.LoginCookie.SameSite,
		HTTPOnly: r.config.Login.LoginCookie.Httponly,
		Path:     r.config.Login.LoginCookie.Path,
	}
	return cookie
}

func IssueGuestCookie(r *Request) *fiber.Cookie {
	sid := r.SessionID()
	jwt := cryptino.GetJWTBasic(sid, r.config.Login.GuestCookie.Expire)
	jwt.Body.Audience = r.config.Login.GuestCookie.JWTAudience

	jwtbuf, err := jwt.Marshal(cryptino.ES256(), r.config.Login.PrivateKey)
	if err != nil {
		r.Logger().Error("IssueGuestCookie: config.login.privatekey is not valid", zap.Error(err))
		return nil
	}

	// set cookie for storing token
	cookie := &fiber.Cookie{
		Name:     r.config.Login.GuestCookie.Name,
		Value:    string(jwtbuf),
		Expires:  time.Now().Add(r.config.Login.GuestCookie.Expire * time.Second),
		Secure:   r.config.Login.GuestCookie.Secure,
		SameSite: r.config.Login.GuestCookie.SameSite,
		HTTPOnly: r.config.Login.GuestCookie.Httponly,
		Path:     r.config.Login.GuestCookie.Path,
	}
	return cookie
}
