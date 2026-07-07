# allino - CONFIG

allino loads server settings from YAML, JSON, and encrypted config files, then initializes built-in services such as Redis, SQL, logging, WebSocket, MCP, jobs, S3, and AI clients.

The default config file name is:

```text
allino.config.yaml
```

The prefix comes from `Config.Prefix`. If you set `Prefix` to `myapp` in Go, allino looks for `myapp.config.yaml` and `myapp.config.enc`.

## Loading Order

`NewServer` applies config in this order. Later sources override earlier values.

1. Embedded defaults from `appsetting_default.yaml`
2. Values passed in the Go `*allino.Config`
3. `Config.ConfigBytes`, when set
4. `<prefix>.config.yaml` and `<prefix>.config.enc` from `Config.ConfigFS`
5. `secrets.config.json` and `secrets.config.enc` from `Config.ConfigFS`
6. The same files from `Config.ConfigDir` on disk
7. CLI flags such as `--config-dir`, `--bind`, and `--debug`

Encrypted `.enc` files require `<PREFIX>_SECRET`, for example `ALLINO_SECRET`. The value must be a base64 raw URL encoded 32-byte key.

## Basic Example

```yaml
appName: My First allino API
description: API server
version: 0.0.1
bind: ":8000"
nowelcome: false
nowrapjson: false
debug: false

trustedproxy:
  trustXForwardedFor: false
  trustXRequestID: false
```

`bind` accepts TCP addresses such as `:8000` and Unix sockets with the `unix:` prefix.

`debug: true` enables debug behavior. Do not use it in production.

## Routing

```yaml
routing:
  fallbacks: ["index.html", "200.html"]
  404error: "/404"
  error: "/error"
```

`fallbacks` is used for static file fallback behavior such as SPA routing.

## Login

```yaml
login:
  publickey: { ... }
  privatekey: { ... }

  oauth:
    expire: 3600
    expire_longterm: 315360000
    querykey: "access_token"
    formkey: "access_token"
    authbearer: true
    jwt_audience: "access_token"

  csrf:
    expire: 3600
    querykey: "csrf_token"
    formkey: "csrf_token"
    jwt_audience: "csrf_token"

  cookie:
    name: allino_login
    expire: 1209600
    secure: false
    httponly: true
    samesite: Lax
    path: "/"
    jwt_audience: "cookie"

  guest_cookie:
    name: allino_guest
    expire: 1209600
    secure: false
    httponly: true
    samesite: Lax
    path: "/"
    jwt_audience: "guest"

  revoke:
    use_login_revoke: false
```

If both `privatekey` and `publickey` are omitted, allino generates a private key at startup.

Use the CLI to create a local secret file:

```sh
yourapp keygen
```

This writes `secrets.config.json` in the config directory. Sensitive settings such as `login.privatekey`, Redis URLs, SQL DSNs, and API keys should live in `secrets.config.json` or `secrets.config.enc`, not in the main config file.

Encrypt a config file:

```sh
yourapp encrypt --file secrets.config.json
```

The encrypted output is `secrets.config.enc`. Deploy the encrypted file with `ALLINO_SECRET`.

## Redis

```yaml
redis:
  url: "redis://localhost:6379/0"
  cluster_url: "redis://localhost:7000"
```

Set either `url` for a single Redis instance or `cluster_url` for a Redis cluster.

## SQL

```yaml
sql:
  driver: "sqlite3"
  dsn: "./foo.db"
  allow_migrate: true
```

`driver` and `dsn` are passed to `database/sql`. Import the driver in your application:

```go
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
	_ "github.com/mattn/go-sqlite3"
)
```

When `allow_migrate` is true, allino executes SQL schemas provided by registered extensions during startup.

## Logging

```yaml
log:
  silent: false
  norequestid: false
  zap:
    addcaller: false
    addcallerskip: 0
    addstacktrace: error
  accesslog:
    - to: file
      path: /path/to/access.log
    - to: file
      rotate:
        cron: "30 * * * *"
        filename: "/path/to/access.log"
        maxsize: 200MB
        maxage: 24d
        maxbackups: 0
        localtime: false
        compress: true
  errorlog:
    - to: stdout
      loglevel: debug
    - to: file
      format: json
      rotate:
        filename: "/path/to/error.log"
        compress: true
```

`to` accepts `stdout`, `stderr`, and `file`. `format: json` uses zap's JSON encoder; otherwise the console encoder is used.

`rotate` uses lumberjack options. `cron` schedules rotation with robfig/cron.

## WebSocket

```yaml
websocket:
  handshakeTimeout: 10s
  subprotocols: ["v1"]
  origins: ["http://localhost:8000"]
  readBufferSize: 1024
  writeBufferSize: 1024
  enableCompression: true
```

These values are used as defaults by `Server.HandleWebsocket`.

## MCP

```yaml
mcp:
  endpoint: "/mcp"
  promptDirs:
    - ./prompts
```

See [MCP.md](./MCP.md) for the MCP endpoint and function exposure behavior.

## Jobs

```yaml
job:
  max_retry: 0
  concurrency: 5
  idle_interval: 1000ms
  lease_duration: 20s
  requeue_interval: 10s
  wait_interval: 700ms
  wait_timeout: 3s
  redis_key_prefix: "allino:key:"
  redis_stream_group_prefix: "allino:group:"
  redis_stream_consumer_prefix: "allino:consumer:"
```

Job modes that use Redis streams, such as fanout and replay modes, require Redis configuration.

## Session

```yaml
session:
  serverid_filename: ".allino-server-id"
  secret: "change-me"
  expire: 24h
  stickey_cookie:
    name: allino_stickey
    path: "/"
    secure: false
    httponly: true
  nodeip: "127.0.0.1"
  nodeip_env: "POD_IP"
  proxyable_hosts: ["127.0.0.1"]
  proxyable_hosts_regex: []
  resources:
    gpu: 1
  redis_prefix: "allino:session"
```

Session settings are used by sticky session functions and session token handling.

## TimeWheel

```yaml
timewheel:
  slots: 32
  tick_interval: 100ms
```

## Sqids

```yaml
sqids:
  alphabet: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
  minlength: 8
  blocklist: []
```

allino initializes `Runtime.Sqids()` from this section.

## S3

Use the default AWS credential chain with a region:

```yaml
s3:
  aws_region: "ap-northeast-1"
```

Or static credentials:

```yaml
s3:
  static:
    base_endpoint: "http://localhost:9000"
    key: "access-key"
    secret: "secret-key"
    session: ""
    use_path_style: true
```

## AI

```yaml
ai:
  default_model: "chatgpt:gpt-4.1"
  tool_max_body_size: 1000
  tool_max_loop: 2
  chatgpt:
    apikey: "sk-..."
    response_api_url: "https://api.openai.com/v1/responses"
```

## HTTPS

```yaml
https:
  certFile: /path/to/server-cert.pem
  keyFile: /path/to/privatekey.pem
```

When `https.certFile` is set, allino serves HTTPS with the configured certificate and key.

## System

```yaml
system:
  disable_validator: false
```

`disable_validator` disables `go-playground/validator` checks for request input.

## Fiber

The `fiber` section maps to `github.com/gofiber/fiber/v2.Config`.

```yaml
fiber:
  bodyLimit: 4194304
  caseSensitive: false
  strictRouting: false
```

Use Fiber's config field names when setting this section.

