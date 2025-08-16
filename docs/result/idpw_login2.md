## PROMPT

([PROMPT TEMPLATE](../README.md#ai-prompt-template-works-well-with-chatgpt) HERE)

Create simple ID Registration and ID/Password login API.

- register api: Get id, pw from form, if id not exist, create and set cookie.
- login api: Get id, pw from form, check its valid, set cookie.

## ChatGPT RESULT

Here's a simple **ID registration** and **ID/password login** API using the `allino` AI-first web framework. It uses form inputs for `id` and `pw`, stores them in `r.SQL` (a pre-initialized `*sql.DB`), and issues login cookies.

---

## 🔐 Assumptions

* You're using `r.SQL` for storing user data.
* Passwords are stored **plaintext** here for simplicity (do **not** use this in production—use bcrypt or similar).
* User data table: `users(id TEXT PRIMARY KEY, pw TEXT)`

---

## 🧾 Register API

```go
type RegisterInput struct {
	ID string `form:"id" validate:"required"`
	PW string `form:"pw" validate:"required"`
}
type RegisterOutput struct {
	Message string `json:"message"`
}

var RegisterHandler = allino.NewTypedAPI(
	"/api/register",
	func(r *allino.Request, in *RegisterInput) (*RegisterOutput, error) {
		var exists int
		err := r.SQL.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", in.ID).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists > 0 {
			return nil, &allino.CodeError{
				StatusCode: 400,
				Code:       "already_exists",
				Msg:        "ID already registered",
			}
		}

		_, err = r.SQL.Exec("INSERT INTO users(id, pw) VALUES (?, ?)", in.ID, in.PW)
		if err != nil {
			return nil, err
		}

		cookie := allino.IssueLoginCookie(r, in.ID, in.ID)
		http.SetCookie(r.Response, cookie)

		return &RegisterOutput{
			Message: "Registered successfully",
		}, nil
	})
```

---

## 🔓 Login API

```go
type LoginInput struct {
	ID string `form:"id" validate:"required"`
	PW string `form:"pw" validate:"required"`
}
type LoginOutput struct {
	Message string `json:"message"`
}

var LoginHandler = allino.NewTypedAPI(
	"/api/login",
	func(r *allino.Request, in *LoginInput) (*LoginOutput, error) {
		var storedPW string
		err := r.SQL.QueryRow("SELECT pw FROM users WHERE id = ?", in.ID).Scan(&storedPW)
		if err != nil {
			return nil, &allino.CodeError{
				StatusCode: 401,
				Code:       "invalid_credentials",
				Msg:        "ID or password incorrect",
			}
		}

		if storedPW != in.PW {
			return nil, &allino.CodeError{
				StatusCode: 401,
				Code:       "invalid_credentials",
				Msg:        "ID or password incorrect",
			}
		}

		cookie := allino.IssueLoginCookie(r, in.ID, in.ID)
		http.SetCookie(r.Response, cookie)

		return &LoginOutput{
			Message: "Login successful",
		}, nil
	})
```

---

## ✅ Optional: SQL Table Creation (SQLite/PostgreSQL-style)

```sql
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	pw TEXT NOT NULL
);
```

---

Let me know if you want to:

* Use password hashing (bcrypt).
* Add CSRF protection.
* Store user data in Redis instead.
* Return CSRF/access tokens on login.

I can adjust the code accordingly.
