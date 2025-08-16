## PROMPT

([PROMPT TEMPLATE](../README.md#ai-prompt-template-works-well-with-chatgpt) HERE)

Create simple ID Registration and ID/Password login API.

- register api: Get id, pw from form, if id not exist, create and set cookie.
- login api: Get id, pw from form, check its valid, set cookie.

## ChatGPT RESULT

Here's a simple **ID registration** and **ID/password login API** using `allino`, leveraging its form handling, CSRF protection, Redis for user storage, and cookie-based session management.

---

### ✅ 1. Define Structures

```go
type RegisterInput struct {
	ID string `form:"id" validate:"required,alphanum"`
	PW string `form:"pw" validate:"required"`
}

type LoginInput struct {
	ID string `form:"id" validate:"required"`
	PW string `form:"pw" validate:"required"`
}

type AuthOutput struct {
	Message string `json:"message"`
}
```

---

### ✅ 2. Redis-backed User Storage

```go
type UserData struct {
	PasswordHash string `json:"password_hash"`
}
```

---

### ✅ 3. Register API

```go
var RegisterAPI = allino.NewTypedAPI[*RegisterInput, *AuthOutput, error](
	"/api/register",
	func(r *allino.Request, input *RegisterInput) (*AuthOutput, error) {
		uid := input.ID
		key := "user:" + uid

		// Check if user already exists
		exists, err := r.Redis.Exists(r.Context(), key).Result()
		if err != nil {
			return nil, fmt.Errorf("redis error: %w", err)
		}
		if exists > 0 {
			return nil, &allino.CodeError{Code: "already_exists", Msg: "User already exists"}
		}

		// Hash the password
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(input.PW)))

		user := UserData{PasswordHash: hash}
		data, _ := json.Marshal(user)
		err = r.Redis.Set(r.Context(), key, data, 0).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to save user: %w", err)
		}

		// Set login session
		cookie := allino.IssueLoginCookie(r, uid, uid)
		http.SetCookie(r.Response, cookie)

		return &AuthOutput{Message: "Registered and logged in"}, nil
	},
)
```

---

### ✅ 4. Login API

```go
var LoginAPI = allino.NewTypedAPI[*LoginInput, *AuthOutput, error](
	"/api/login",
	func(r *allino.Request, input *LoginInput) (*AuthOutput, error) {
		uid := input.ID
		key := "user:" + uid

		// Fetch from Redis
		data, err := r.Redis.Get(r.Context(), key).Bytes()
		if err == redis.Nil {
			return nil, &allino.CodeError{Code: "not_found", Msg: "User not found"}
		} else if err != nil {
			return nil, fmt.Errorf("redis error: %w", err)
		}

		var user UserData
		_ = json.Unmarshal(data, &user)

		// Check password
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(input.PW)))
		if user.PasswordHash != hash {
			return nil, &allino.CodeError{Code: "invalid_password", Msg: "Invalid credentials"}
		}

		// Issue cookie
		cookie := allino.IssueLoginCookie(r, uid, uid)
		http.SetCookie(r.Response, cookie)

		return &AuthOutput{Message: "Logged in"}, nil
	},
)
```

---

### ⚙️ Optional: Register API with CSRF Token Protection

To protect `POST` routes (like `/register` or `/login`) from CSRF:

```go
&allino.HandlerOption{
	Path:        "/api/register",
	Method:      "POST",
	ContentType: "application/json",
}
```

Then the form must include a valid CSRF token, which you can issue via:

```go
token := allino.IssueCSRFToken(r, uid)
```

---

Let me know if you also want a **logout API**, **password change**, or **HTML form version** of these handlers.
