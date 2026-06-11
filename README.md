# allino

[![Go Report Card](https://goreportcard.com/badge/github.com/wh-kuromai/allino)](https://goreportcard.com/report/github.com/wh-kuromai/allino)
[![Go Reference](https://pkg.go.dev/badge/github.com/wh-kuromai/allino.svg)](https://pkg.go.dev/github.com/wh-kuromai/allino)


**AI-first web framework for Go**  
Let your AI generate your apps using best-practice OSS – automatically.

---

## ✨ Features

- **AI-Ready API Definition**: Handler signatures, validation, logging, database access, and authentication are all pre-wired for AI code generation
- **[AI-optimized prompt template](#ai-prompt-template-works-well-with-chatgpt)** for instant code generation
- **Strongly-typed API definition** using Go generics
- **Automatic input validation** via `go-playground/validator`
- **Built-in Authentication & Authorization support** with native CSRF protection
- **Auto-generated OpenAPI docs** for your API
- **Popular OSS integrations via a [single JSON config](./docs/CONFIG.md)**: `fiber`, `go-redis`, `sql`, and more
- **Logging** : Apache Combined access-log, structured error logging with `zap`, log rotation via `lumberjack` and `cron`
- **Test code generation** with **[this prompt](./docs/TEST.md#ai-prompt-template-for-test-works-well-with-chatgpt)**
- **Single Page Application (SPA) Support** : Seamlessly serve `react`, `vue`, `svelte` or any static website.
- **Legacy-friendly**: Drop-in support for existing http.Handler code
- **Out-of-the-box CLI** with beautiful help and route listing
---

## 🚀 Getting Started

Getting started takes just a few steps:

```bash
$ go get github.com/wh-kuromai/allino@latest
```

First, install the package.  
Then, your entry point can be as simple as:

```go
package main

import (
	"github.com/wh-kuromai/allino"
	_ "github.com/wh-kuromai/allino/example/handlers"
)

func main() {
	allino.RunCLI(nil)
}
```

You might be thinking:

> “But wait — don’t I need a giant config file or to manually register all my handlers?”

Nope — not at all.
allino automatically registers all handlers declared in imported packages, so you don’t need to do any explicit registration unless you want to.
If you prefer to register handlers manually, that’s also fully supported.

---

Let’s try creating a simple API:

```go
// AI-friendly type definition
type HealthcheckAPIInput struct {
	Echo string `query:"echo"`
}
type HealthcheckAPIOutput struct {
	Status  string    `json:"status"`
	Echo    string    `json:"echo,omitempty"`
	StartAt time.Time `json:"startAt"`
}

var HealthcheckAPITypedHandler = allino.NewTypedAPI("/api/healthcheck",
    func(r *allino.Request, param *HealthcheckAPIInput) (*HealthcheckAPIOutput, error) {
        // Actual API logic here.
		return &HealthcheckAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config.StartAt,
		}, nil
	})
```

It might look a little unfamiliar at first, but it’s built on top of a simple generic helper function:

```go
func NewTypedAPI[T, U any, E error](path string, handlefunc func(r *Request, input T) (output U, err E)) TypedHandler
```

You can use any Go types for input and output.
The framework automatically parses path/query/form parameters into your struct, validates them using go-playground/validator
, and passes the fully-populated struct into your handler.

The allino.Request includes fiber.Ctx, a zap.Logger, and a redis.Client — everything you need to start building real-world logic right away.

---

Once your first API is ready, let’s boot the server.

```bash
$ go run main.go
NAME:
   allino - AI-first web framework server
...
```

You’ll see a help message like this.
Now, before actually running the server, let’s try something magical:

```bash
$ go run main.go gendoc routes
GET /api/healthcheck 
```

That’s right — all registered routes are automatically listed along with their metadata.
And if you want full documentation? Just run:

```bash
$ go run main.go gendoc openapi
```

See [CLI Output Samples](./docs/CLI.md) for full examples.

You may be surprised by the accuracy and completeness.
Finally, let’s start the server for real:

```bash
$ go run main.go serve
```

You're now running your first allino server!

### 🧠 Can AI generate code for a new framework like this?

Yes — that's exactly what allino is designed for.

You can use our compact, **AI-optimized prompt template** to describe your desired API,
and get fully working code instantly, powered by many powerful OSS tools like `fiber`, `zap`, `go-redis`, `go-playground/validator` and more.

This means you don't need to explain the whole framework — just paste your API idea along with the prompt, and AI can take it from there.

---
## 📋 AI Result

Examples of AI-generated results using **[this prepared AI Prompt Template](#ai-prompt-template-works-well-with-chatgpt)**.

| Description | Type | AI | Result | 
| --- | --- | --- | --- |
| Create simple QR code API | api, binary | ChatGPT-4o | ✅[Result](./docs/result/qrcode.md) |
| Create simple short URL API | api, redis, path-param, redirect | ChatGPT-4o | ✅[Result](./docs/result/shorturl.md) |
| Create simple ID Registration and ID/Password login API | 2-apis, redis, sql, login, cookie | ChatGPT-4o | ✅[1](./docs/result/idpw_login.md), ✅[2](./docs/result/idpw_login.md) |

... and [more results.](./RESULT.md)

---
## AI Prompt Template (works well with ChatGPT)

You can paste the following after your API idea, and get working `allino` code instantly:

```go
// allino AI-first web-framework
//   allino is an AI-first web framework that leverages Go generics to define clear, type-safe input/output structures.
//   It enables automatic validation, human-readable handler signatures, and auto-generation of OpenAPI documentation.
//   By integrating popular OSS such as `fiber`, `redis`, `zap`, and `go-playground/validator`,
//   it improves compatibility with AI-generated code, making it easier for LLMs to produce reliable implementations.
//   Use this framework to implement the API requested by the user.
// Input: 
//   - Fields are populated in order: path parameters → query parameters → form values. 
//   - Validated using go-playground/Validator. Then passed to the handler function.
//   - Field types can be string, []byte, int, *multipart.FileHeader, or other primitive types.
//   - If no input is needed, use `any` as the input type to indicate that no data is required.
// Output:
//   JSON: 
//     - Return a struct or a pointer to a struct. Automatically wrapped as {"data":{...}}, marshaled via json.Marshal, and written to response.
//     - Avoid using `any` as the return type in JSON APIs, as it prevents OpenAPI schema generation.
//   HTML:
//     - If returning string or []byte, it will be written directly to the response.
//     - If returning any other object and HandlerOption.HTMLTemplate is set,
//       the value will be passed to html/template as the template data and rendered.
//     - If an unsupported type is returned without a template, it will be converted to `string` via `fmt.Sprint`.
// Error:
//   JSON:
//     - If returning an error, it is wrapped as {"error": {...}} and written with default status code.
//     - If returning CodeError, it is marshaled and written with specified StatusCode.
//   HTML:
//     - If returning error, it redirects to a default error page.
//     - If returning RedirectError, sends err.StatusCode and redirects to err.Location.
package allino //github.com/wh-kuromai/allino
func NewTypedHandler[T, U any, E error](options HandlerOption, handlefunc func(r *Request, input T) (output U, err E)) *GenericTypedHandler[T, U, E]
type Request struct {}
func (r *Request) Fiber() *fiber.Ctx // only avaiable via http request. (nil if virtual)
func (r *Request) Logger() *zap.Logger // use this for logging, inited via config.
func (r *Request) Redis() redis.UniversalClient // go-redis Client, inited via config.
func (r *Request) SQL() *sql.DB // pre-Opened sql Client, inited via config.
func (r *Request) S3() *s3.Client // AWS SDK v2 S3 client, inited via config. (MinIO/AWS)
func (r *Request) HttpClient() *http.Client // inited shared http client.
// User() checks and validates using Cookie, Authorization header, X-CSRF-Token header or `csrf_token` form data.
// Returns uid (database key), display name, and sets writable=true only when a write-intent credential is presented 
// (e.g., Authorization header or an explicit token in form/query/header) and CSRF validation succeeds. 
// Otherwise the user is treated as read-only. 
func (r *Request) User() (uid, displayname string, writable bool, err error)
// RequestID() returns unique generated xid, X-Request-ID header if config.trustedproxy.trustXRequestID is true,
// async/dispatch JobID, or fanout/replay/replayall redis stream MessageID.
func (r *Request) RequestID() string
// SessionID() returns the session ID from the guest cookie.
// If the cookie is missing, it generates a new ID and sets it via fiber.Ctx.
func (r *Request) SessionID() string
func (r *Request) Context() context.Context
// NewCodeError makes error returning specified http status code.
func NewCodeError(statusCode int, code string, msg string) error
// fmt.Errorf version of NewCodeError. (compatible with errors.Is/errors.As)
func NewCodeErrorf(statusCode int, code string, format strng, a ...any) error
// or implememnts HttpError interface to your own error for specify status code.
type HttpError interface {
	StatusCode() int
}
// NewStatusError makes error returns only status code.
func NewStatusError(status int) error
// NewRedirectError makes error performing redirect. Since allino requires typed responses via generics, redirects are treated as error values.
func NewRedirectError(status int, location string) error
// IssueCSRFToken issues a short-lived token used to protect write operations from CSRF attacks.
// by default, this token should be added to `csrf_token` query or form parameter.
func IssueCSRFToken(r *Request, uid string) string
// IssueAccessToken issues a short-lived token used to API access.
// Optional custom JWT claims can be provided; they can later be retrieved via struct fields tagged with `jwt:"key"`.
func IssueAccessToken(r *Request, uid, displayname string, jwt_custom_claims ...map[string]any) string
// IssueLoginCookie issues a login cookie for user authentication.
func IssueLoginCookie(r *Request, uid, displayname string, jwt_custom_claims ...map[string]any) *fiber.Cookie
type HandlerOption struct {
	Path string
	Method string // "GET", "POST", etc.
	SubMethod []string
	ContentType string // e.g. "application/json"
	CORS bool // if true, add Access-Control-Allow-Origin:* to OPTIONS request
	NoWrapJSON bool // if true, do not pack {"data":{...}} or {"error":{...}}, ignore when content-type is not json.
  Summary string // OpenAPI Operation Summary
	Description string // OpenAPI Operation Description
  ResponseStatusCode int // default is 200. Also used as the response code in the OpenAPI spec.
	ErrorStatusCode    int // default is 400.
	HTMLTemplate       string // html/template text
	OnInit     func(s *Server, virtual *Request) error // Init code for this handler, use Request for DB or Logger.
	OnShutdown func(s *Server, virtual *Request) error // Finalize code for this handler, use Request for DB or Logger.

  Internal bool // If true, this handler is not exposed as an HTTP endpoint.
  Name    string // Logical name of this handler. Required when using Job mode.
  Version string // Semantic version of the handler (e.g. "1.0.0"). Optional.

  Class string // Make this handler class method.

  // Cron expression to schedule this handler.
  // custom `?` specifier for random number, `N-M?` for random number between N and M.
  Cron string 

  // JobMode defines the execution behavior of this handler.
  // Note: Certain modes require the handler to be idempotent. Allino caches/stores
  // results using a key: Name + Version + hash(input).
  // 
  //   "" (Default) : Standard synchronous execution.
  //   "cache"      : Caches the result based on input JSON; returns the cached result if available.
  //   "dedupe"     : Ensures only one execution runs concurrently for the same input; returns allino.ErrJobDuplicated if dupulicated. (Requires idempotency)
  //   "once"       : Ensures the handler runs only once per unique input; Subsequent calls return allino.ErrJobDuplicated. (Requires idempotency)
  //   "memoized"   : Ensures the handler runs only once per unique input; Subsequent calls wait complete and return cached result. (Requires idempotency)
  //
  // Async job mode:
  //   "async"      : Runs the handler asynchronously. (Internal=true only)
  //   "dispatch"   : Hybrid of Async + Cache. Returns a cached result synchronously if found; otherwise, enqueues as 'async'. (Requires idempotency)
  //
  // Broadcast job mode:
  //   "fanout"    : deliver new jobs, retains last node's processed position.
  //   "replay"    : replay all jobs, retains last node's processed position.
  //   "replayall" : replay all jobs every restart. (for in-memory state)
  JobMode string
  Job JobOption
  
  // Session configures request-scoped shared state.
  Session allino.Session
}
type JobOption struct {
	Priority int // optional. Priority of the handler's jobs. Higher values indicate higher priority.
  Interval time.Duration // optional. Approximate interval between executions (used in async/dispatch mode).
  CacheExpire time.Duration // optional. Cache expiration duration. Persistent if 0 (default).

  // Upgrade old input/output data into current version instance.
	OnInputUpgrade  func(version string, old_input_at time.Time, old_input []byte) (bool, any)
	OnOutputUpgrade func(version string, old_output_at time.Time, old_output, old_error []byte) (bool, any, error)

  // OnReplayInit returns the stream position to start replaying after. optional.
  // replayAfter MessageID can be retrieved by r.RequestID()
	OnReplayInit func() (replayAfter string, err error) 
  // ReplayTTL automatically removes expired jobs/events from the stream. optional. 
	ReplayTTL    time.Duration 
}

type IdempotentRequest interface {
	IdempotencyKey() string // Override caches/stores key, when input struct of the handler implements this.
}

type SessionOption struct {
	// Type defines the session backend.
	//   "" or "redis":
	//       Standard distributed session backed by Redis.
	//
	//   "sticky":
	//       Keeps the session object entirely in memory on a single node.
	//       The object is never serialized.
	//
	//       Requests are automatically routed to the owning node,
	//       making this suitable for stateful/non-serializable resources
	//       such as browser automation, AI agents, websocket state,
	//       or other long-lived in-memory objects.
	Type string
  // Name identifies the session group.
  // Handlers sharing the same Name will access the same session instance.
  Name string
  // UseResource declares scheduler resource consumption for sticky sessions.
  // When a sticky session is created, the scheduler allocates the specified
  // resources on the selected node.
  // Available node resources are configured via `config.session.resources`.
  UseResource map[string]int
}

// WithSession retrieves/create a typed session instance associated with the request, then call callback within sync.Mutex lock.
// A session ID is automatically issued and managed via cookies.
func WithSession[S any](r *Request, fn func(*S) error) error

// Call executes the handler.
// This can be used inside another handler or from application code.
func (rw *GenericTypedHandler[T, U, E]) Call(r *Request, input T) (output U, err error)

func (r *Request) MarkAbort() // aborts the current execution (prevent caching or storing result/error)
func (r *Request) MarkRequeue() // aborts the execution, and schedules a retry with the same input after a short delay.
func (r *Request) MarkRequeueAt(waitsec int) // aborts the execution, and schedules a retry with the same input after the specified seconds.

// Global, TTL-trimmed redis stream with in-memory TTL-rotated radix-tree and exact-match map revoke system
// The in-memory radix tree and map are safely restored during server initialization.
// A scope can be revoked either by exact string match or by prefix match using a trailing wildcard (e.g. "some:string:*").
// TTLs for exact and prefix revocations can be configured via server config.
func (r *Request) Revoke(scope string, reason string) 
func (r *Request) IsRevoked(scope string, issuedAt time.Time) (revoked bool, reason string) 

func GetClassHandlers(class string) []TypedHandler

// EXAMPLE
import (
	"github.com/wh-kuromai/allino"
)
type SampleAPIInput struct {
	Echo string `query:"echo" validate:"required"` // Required query parameter (validated by go-playground/validator)
	Uid  string `path:"uid"` // Supported parameter tag: path:"path", query:"key", form:"key", jwt:"key" (populated only from a successfully verified JWT Claims), cookie:"name", header:"name"
  Version string `query:"ver" default:"v1"`   // Default values (applied before binding; empty input overwrites)
  // Body SampleAPIInputJSONBody `post:"json"` // Automatically binds JSON body to this field. (json.Unmarshal(body, &param.Body))
  // CLIFilePath string `cli:"path"` // CLI variables (yourapp run YourHandler --set path=abc.txt)
  
  // `inject` tag will make this handler works like class method.
  //  extensions are responsible for 
  //    caller send instanceid as query/form/path input string
  //    -> send to extension 
  //    -> extension retreive data, check ACL, then return data.
  //    -> unmarshal into this param.
  // Instance SampleClass `query:"instanceid" inject:"extensionName:action"` 
}
type SampleAPIOutput struct {
	Echo    string    `json:"echo,omitempty"`
}
var SampleAPITypedHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Path:        "/api/:uid/userinfosample", // `fiber` style path-parameter allowed. if you don't need :uid, remove it.
		Method:      "GET",
		ContentType: "application/json", // JSON or HTML, content-type will be sent automatically.
	},
	func(r *allino.Request, param SampleAPIInput) (*SampleAPIOutput, error) { // Both value and pointer params work; prefer value for small input structs (fewer allocs/escapes), and prefer pointer for output (short returns `return nil, err`).
		return &SampleAPIOutput{
			Echo:    param.Echo,
		}, nil
	})
```

## FAQ

### 💡 Do I have to migrate my existing code?

No, you can also use `http.Handler` as it is.

allino lets you incrementally adopt the framework — Even legacy `http.Handler` implementations are fully supported.

```go
func yourHandler(w http.ResponseWriter, r *http.Request) {
    ...
}

func main() {
    allino.RunCLI(&allino.Config{
        OnInit: func(server *allino.Server) {
            
            // register your legacy api handler
            server.HandleFunc("GET", "/api/legacy", yourHandler)

        }
    })
}
```

Or, if you want to integrate your legacy handler into `gendoc` and OpenAPI generation:

```go
func yourHandler(w http.ResponseWriter, r *http.Request) {
    ...
}

func main() {
    allino.RunCLI(&allino.Config{
        OnInit: func(server *allino.Server) {
            
            server.TypedHandleFunc(allino.HandlerOption{
                Path:        "/api/legacy",
                Method:      "GET",
                ContentType: allino.JSON,

                // Hints are used for generating OpenAPI Response schemas.
                // InputTypeHint : &YourInputType{},
                // OutputTypeHint: &YourOutputType{}, 
                // ErrorTypeHint : &YourErrorType{},

                // Uncomment this, if your API is not wrapped with {"data":{...}} or {"error":{msg:"..."}}
                // NoWrapJSON : true, 
            }, yourHandler)

        }
    })
}
```

This makes it easy to incrementally migrate your existing API server to allino, without needing to rewrite everything from scratch.

---
### 🧪 Want to generate even more perfect OpenAPI docs?

Previously, we used `allino.NewTypedAPI`, but if you switch to `allino.NewTypedHandler`, you can configure various options like:

- Add `Summary` and `Description` to the OpenAPI docs
- Enable CORS: Send `Access-Control-Allow-Origin: *` for OPTIONS requests
- Disable JSON wrapping: Prevent output from being wrapped in `{"data":{...}}` or `{"error":{...}}`

```go
var HealthcheckAPITypedHandler = allino.NewTypedHandler(
	allino.HandlerOption{
		Summary:     "Simple health check API",
		Path:        "/api/healthcheck",
		Method:      "GET",
		ContentType: allino.JSON,
	},
	func(r *allino.Request, param *HealthcheckAPIInput) (*HealthcheckAPIOutput, error) {
		return &HealthcheckAPIOutput{
			Status:  "OK",
			Echo:    param.Echo,
			StartAt: r.Config.StartAt,
		}, nil
	})
```

---

## 📊 Benchmark

| Software | githubAPI | | gplusAPI | | parseAPI | |
|----------|-----------------|--------|-----------------|--------|-----------------|--------|
| fiber    | 101,496 req/sec | 1.06ms | 112,741 req/sec | 0.96ms | 108,303 req/sec | 0.99ms |
| allino (fiber +validation,etc)   |  86,738 req/sec | 1.30ms |  94,450 req/sec | 1.22ms |  95,382 req/sec | 1.18ms |
| echo     |  84,966 req/sec | 1.33ms |  80,391 req/sec | 1.42ms |  79,833 req/sec | 1.44ms |
| gin      |  84,373 req/sec | 1.37ms |  83,669 req/sec | 1.36ms |  83,033 req/sec | 1.45ms |

use test-data from https://github.com/vishr/web-framework-benchmark

---

## 💡 Inspiration

I created this framework with questions like:

- How can we get AI to write code that uses fiber, zap, and go-redis properly?
- How should input/output be described for AI?
- How can we keep the generated code compact?
- How do we describe previously generated code to AI?

Most existing frameworks prioritize human readability and flexibility. But for AI, that often makes it unclear which tools are allowed. For example, when AI needs to use `go-redis`, it might start by opening a database connection inside the handler. If you say “use `zap`”, the AI might begin by writing `zap` initialization code. And when it needs a user ID, it may generate a fake placeholder instead of using proper authentication.

allino solves this by encoding those expectations into the framework itself.

Once you've built a few APIs, try running 

```bash
$ go run main.go gendoc openapi
```

and giving the result to an AI — it will likely respond:

“This is great! Maybe we can also add an endpoint like X?”

That’s when things get really fun. ✨
And the best part? The AI will thank *you* for making its life easier.