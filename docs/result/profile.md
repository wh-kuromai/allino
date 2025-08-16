## PROMPT

([PROMPT TEMPLATE](../README.md#ai-prompt-template-works-well-with-chatgpt) HERE)

Create user profile get and edit apis. I want to store firstname, lastname, address, phonenumber and email.

## ChatGPT RESULT

Here’s how you can implement **User Profile GET and EDIT APIs** using the **allino AI-first web framework**, with Redis as persistent storage via `UserData`.

We’ll define:

1. A struct for the user profile.
2. A **GET API** to fetch the profile.
3. A **POST API** to update the profile.

---

### ✅ 1. Define the UserProfile struct

```go
type UserProfile struct {
	FirstName   string `json:"firstname,omitempty"`
	LastName    string `json:"lastname,omitempty"`
	Address     string `json:"address,omitempty"`
	PhoneNumber string `json:"phonenumber,omitempty"`
	Email       string `json:"email,omitempty"`
}
```

---

### 📥 2. GET /api/user/profile

**Returns** the current user's stored profile.

```go
var GetUserProfileAPI = allino.NewTypedAPI[struct{}, *UserProfile, error](
	"/api/user/profile",
	func(r *allino.Request, _ struct{}) (*UserProfile, error) {
		uid, _, writable, err := r.User()
		if err != nil || !writable {
			return nil, allino.CodeError{StatusCode: 401, Code: "unauthorized", Msg: "User not authorized"}
		}

		var profile UserProfile
		err = r.GetUserDataRedis(uid, &profile)
		if err != nil {
			return nil, allino.CodeError{StatusCode: 500, Code: "redis_error", Msg: "Failed to load user data"}
		}

		return &profile, nil
	},
)
```

---

### 📝 3. POST /api/user/profile

**Updates** the user's profile fields.

```go
type UpdateProfileInput struct {
	FirstName   string `form:"firstname" validate:"required"`
	LastName    string `form:"lastname" validate:"required"`
	Address     string `form:"address" validate:"required"`
	PhoneNumber string `form:"phonenumber" validate:"required,e164"` // You can customize this validation
	Email       string `form:"email" validate:"required,email"`
}

var EditUserProfileAPI = allino.NewTypedAPI[*UpdateProfileInput, struct{}, error](
	"/api/user/profile",
	func(r *allino.Request, input *UpdateProfileInput) (struct{}, error) {
		uid, _, writable, err := r.User()
		if err != nil || !writable {
			return struct{}{}, allino.CodeError{StatusCode: 401, Code: "unauthorized", Msg: "User not authorized"}
		}

		profile := UserProfile{
			FirstName:   input.FirstName,
			LastName:    input.LastName,
			Address:     input.Address,
			PhoneNumber: input.PhoneNumber,
			Email:       input.Email,
		}

		err = r.SetUserDataRedis(uid, &profile)
		if err != nil {
			return struct{}{}, allino.CodeError{StatusCode: 500, Code: "save_error", Msg: "Failed to save user data"}
		}

		return struct{}{}, nil
	},
)
```

---

### 🛡️ Notes:

* `User()` authenticates the user and checks CSRF/auth headers.
* All fields are validated via [go-playground/validator](https://pkg.go.dev/github.com/go-playground/validator).
* Responses follow allino conventions: `{"data":{...}}` or `{"error":{...}}`.
* You may extend this to accept JSON input by changing `form:"..."` to `post:"json"` and adjusting content-type in `HandlerOption`.

Would you like the same APIs with HTML templates too (using `NewTypedUI`)?
