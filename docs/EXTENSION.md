# allino - Extension

allino extensions add framework-level behavior around server startup, function registration, request execution, authentication, injection, SQL schema generation, and CLI commands.

Extensions are registered globally with `allino.NewExtension`.

```go
var MyExtension = allino.NewExtension[MyConfig, MyOption](
	"myext",
	&allino.ExtOption{
		OnInit: func(s *allino.Server, virtual *allino.Runtime) error {
			return nil
		},
	},
)
```

## Configuration

The first generic type is the extension config type.

```go
type MyConfig struct {
	Enabled bool   `json:"enabled"`
	Prefix  string `json:"prefix"`
}

var MyExtension = allino.NewExtension[MyConfig, any]("myext", nil)
```

Configure it with a top-level YAML section whose name matches the extension name:

```yaml
myext:
  enabled: true
  prefix: "demo"
```

During normal, non-secret config loading, allino decodes `$.myext` into `MyExtension.Config`.

Encrypted and secret config files are loaded with the secure flag and do not update extension config. Keep extension config in the main app config, or pass initial extension config from Go:

```go
s, err := allino.NewServer(&allino.Config{}, map[string]any{
	"myext": MyConfig{Enabled: true},
})
```

Use `any` as the config type when the extension does not need config:

```go
var MyExtension = allino.NewExtension[any, any]("myext", nil)
```

## Function Options

The second generic type is per-function extension metadata.

```go
type MyFunctionOption struct {
	Mode string `default:"read"`
}

var MyExtension = allino.NewExtension[any, MyFunctionOption]("myext", nil)
```

Set function-specific metadata with `Option.WithExt`:

```go
var Example = allino.NewFunction(
	allino.Option{
		Name:        "example",
		ContentType: allino.JSON,
	}.WithExt(MyFunctionOption{Mode: "write"}),
	func(r *allino.Runtime, input *Input) (*Output, error) {
		return &Output{}, nil
	},
)
```

Read it inside extension hooks:

```go
meta, userSet := MyExtension.OptionExt(opt)
```

`userSet` is true when the function explicitly called `WithExt`. Defaults from `default` tags are applied when possible.

## Lifecycle Hooks

`ExtOption` supports these lifecycle hooks:

| Hook | When it runs |
| --- | --- |
| `OnInit` | After core services such as logger, Redis, SQL, HTTP client, Sqids, S3, and TimeWheel are initialized |
| `OnFunctionInit` | For each registered function during `serveInitOnly` |
| `OnServe` | After function initialization, before serving work continues |
| `OnShutdown` | During graceful shutdown after function shutdown hooks |

Built-in examples:

- `JobExtension` initializes SQL and Redis job backends with `OnFunctionInit`, finalizes Redis stream setup with `OnServe`, and registers handler names for CLI calls.
- `SessionExtension` creates sticky session support when a function sets `Option.Session.Type` to `sticky`.
- `MCPExtension` registers the MCP HTTP endpoint when `Option.MCP` or `mcp.promptDirs` is configured.

## Request Hooks

Extensions can intercept function execution:

| Hook | Behavior |
| --- | --- |
| `RequestHandler` | Runs after input parsing and before the function handler. Return `consumed=true` to skip the handler. |
| `ResponseHandler` | Runs before the function's own response handler and the default content-type response writer. Return `consumed=true` to write the response yourself. |
| `ErrorHandler` | Runs before the function's own error handler and the default error writer. Return `consumed=true` to write the error response yourself. |

Execution order for HTTP functions is:

1. Parse and validate input
2. Function `Option.RequestHandler`
3. Extension `RequestHandler`
4. Function handler, unless consumed
5. Extension `ErrorHandler` or `ResponseHandler`
6. Function `Option.ErrorHandler` or `Option.ResponseHandler`
7. Default content-type response writer

## Authorization Hook

`OnAuthZ` runs after allino verifies a JWT and before `Runtime.User()` returns the authenticated user.

```go
OnAuthZ: func(r *allino.Runtime, jwt *cryptino.JSONWebToken) (*cryptino.JSONWebToken, error) {
	return jwt, nil
}
```

Return an error to reject the login. The `revoker` extension uses this hook to reject revoked JWTs.

## Injection Hook

Struct fields can request extension injection with the `inject` tag:

```go
type Input struct {
	ID   string      `path:"id"`
	Node NodeDetails `inject:"objects:read"`
}
```

When allino parses input, it collects `InjectionTarget` values and calls the matching extension's `OnInjection` hook.

```go
OnInjection: func(r *allino.Runtime, targets []*allino.InjectionTarget) error {
	for _, target := range targets {
		// target.Input is the source string.
		// target.Action is the suffix after "extension:".
		// target.Reference points to the field to populate.
	}
	return nil
}
```

The `objects` extension uses this hook to resolve object IDs, check ACLs, and unmarshal metadata into the target field.

## SQL Schema

Extensions can provide SQL schema:

```go
SQLSchema: func(driver string) string {
	return `CREATE TABLE IF NOT EXISTS example (id TEXT PRIMARY KEY);`
}
```

The CLI can print all registered extension schemas:

```sh
yourapp sqlschema
```

At startup, allino executes extension SQL schemas when SQL is configured and `sql.allow_migrate` is true or omitted.

## CLI Commands

Extensions can add Cobra commands:

```go
CLICommands: []*cobra.Command{
	{
		Use:   "myext",
		Short: "Run my extension command",
		Run: func(cmd *cobra.Command, args []string) {
			// command logic
		},
	},
}
```

Extension commands are added to the root CLI when `NewCLI` is created.

## Built-in Extensions

allino registers several built-in extensions:

| Extension | Purpose |
| --- | --- |
| `job` | Job backend initialization, worker metadata, direct CLI calls |
| `session` | Sticky session support |
| `mcp` | MCP HTTP endpoint and mounted Markdown prompts |

Additional packages can register their own extensions during package initialization, such as `ext/objects` and `ext/revoker`.

