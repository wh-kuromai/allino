
# allino - CONFIG

## ✍ How to configure the allino server?

allino integrates various popular open-source libraries using a single YAML configuration file.

You don't need to write any code to start using features like `fiber`, `go-redis`, `sql`, `zap`, `lumberjack`, and `cron`.
Just add the following two lines to your YAML config, and you'll be able to access a `*redis.Client` instance via `r.Redis` in any handler.

```yaml
redis:
  url: "redis://localhost:6379/0"
```

## YAML Setting

```yaml
# Application metadata
appName: My First allino API   # Name of your app, also used for OpenAPI doc `title`
version: 0.0.1                 # Version

# Server binding information
bind: ":8000"                  # Address to bind (e.g., ":8000", "0.0.0.0:80", or "unix:/tmp/your.sock")
trustXForwardedFor: false      # True if this server works behind trusted proxy 
nowrapjson: false              # if true, do not pack {"data":{...}} or {"error":{...}} 
debug: false                   # Enable debug features (logging, verbose output, etc.)

# Routing configuration (optional)
routing:
  fallbacks: ["index.html", "200.html"]   # Files to serve when the requested file is not found (SPA support).
  404error: "/404"                        # Custom path to serve for 404 Not Found errors.
  error: "/error"                         # Path to serve for generic application errors.

# Login configuration
#   This section defines security settings related to authentication,
#   such as cookie/token expiration, and cryptographic keys for login flows.
#   You can provide either:
#     - a private key (for servers that initiate login sessions), or
#     - a public key (for servers that only perform cookie/token validation).
#
#   If omitted, a private key will be generated automatically at startup.
#
#   In production environments, it is recommended to manage your JWT keys and other sensitive data securely
#   using the following procedure:
#
#     1. Generate key material by running:
#          yourapp keygen
#        This will create a `secrets.config.js` file containing your JWT keys.
#
#     2. Move any sensitive settings (such as `login`, `redis`, `sql`, etc.)
#        from your main config file into `secrets.config.js`.
#
#     3. Encrypt the file by running:
#          yourapp encrypt secrets.config.js
#        This will generate an encrypted `secrets.config.enc` file and print a decryption key.
#        Be sure to **store the decryption key securely**.
#
#     4. When deploying the app:
#          - Include only `secrets.config.enc` in your configuration directory.
#          - Set the decryption key using the `ALLINO_SECRET` environment variable.
#
#     5. Keep the original `secrets.config.js` in a safe location and never deploy or commit it to version control.
#
#   (Note: Never commit unencrypted private keys or secret files to your repository.)
login:
  publickey: { ... }   # Use yourapp keygen to generate proper public/private keys here.
  privatekey: { ... }

  oauth:
    expire: 3600                    # AccessToken expire in sec. (time.ParseDuration style also supported)
    expire-longterm: 315360000      # APIToken expire in sec. (time.ParseDuration style also supported)
    authbearer: true                # allow Authorization header
    jwt_audience: "access_token"    # jwt audience field value
  csrf:
    expire: 3600
    querykey: "csrf_token"          # query key for CSRF Token (delete if you dont accept token in query)
    formkey: "csrf_token"           # form key for CSRF Token
    jwt_audience: "csrf_token"      # jwt audience field value
  cookie:
    name: allino_login              # Set-Cookie name
    expire: 1209600                 # Set-Cookie expire in sec. (time.ParseDuration style also supported)
    secure: false                   # Set-Cookie secure field
    httponly: true                  # Set-Cookie httponly field
    samesite: Lax                   # Set-Cookie samesite field
    path: "/"                       # Set-Cookie path field
    jwt_audience: "cookie"          # jwt audience field value
  guest_cookie:
    name: allino_guest              # Set-Cookie name
    expire: 1209600                 # Set-Cookie expire in sec. (time.ParseDuration style also supported)
    secure: false                   # Set-Cookie secure field
    httponly: true                  # Set-Cookie httponly field
    samesite: Lax                   # Set-Cookie samesite field
    path: "/"                       # Set-Cookie path field
    jwt_audience: "guest"           # jwt audience field value


# Redis configuration (optional)
#   In production, avoid hardcoding credentials. 
#   Move the redis.url setting into your encrypted `secrets.config.js` file.
redis:
  url: "redis://localhost:6379/0"

# SQL configuration (optional)
#   Database DSN strings often contain passwords.
#   To protect them, move the dsn setting into the encrypted secrets file in production.
sql:
  driver: "mysql"
  dsn: "user:password@tcp(localhost:3306)/exampledb?charset=utf8mb4&parseTime=True&loc=Local"

# Logging configuration (optional)
log:
  silent: false       # minimize output
  norequestid: false  # disable auto-add `request-id` to your zap log
  accesslog:
  - to : file                                 # "stdout", "stderr" or "file"
    path: /path/to/your/simple_accesslog.txt  # filename for log file. (only if you don't use rotate)

  - to : file                                 # "stdout", "stderr" or "file"
    # `rotate` controls file rotation using natefinch/lumberjack.
    # The same fields and behavior apply for both `accesslog` and `errorlog`.
    # In addition to lumberjack's standard options, you can set `cron` (handled by robfig/cron)
    # to schedule rotation independently of file size.
    # see https://github.com/natefinch/lumberjack/tree/v2.0.0
    # see https://pkg.go.dev/github.com/robfig/cron/v3@v3.0.1
    rotate:                                   # settings for `natefinch/lumberjack` and `robfig/cron`
      cron: "30 * * * *"                      # cron setting for log rotation by `robfig/cron` (ex. every hour on the half hour)
      filename: "/path/to/your/accesslog.txt" # filename for log file. rotated file will be created on same dir.
      maxsize: 200MB                          # MaxSize is the maximum size in megabytes before rotation. Defaults to 100 MB. Supports units: TB, GB, MB, KB, B
      maxage: 24d                             # MaxAge is the maximum retention period based on the timestamp in the filename. Rounded up to the nearest whole day.
      maxbackups: 0                           # MaxBackups is the maximum number of old log files to retain. Defaults to unlimited (0).
      localtime: false                        # use localtime for filename.
      compress: true                          # enable gzip compression for rotated files.
  errorlog:
  - to : stdout                               # `stdout`, `stderr` or `file`
    loglevel: debug                           # zap log level (`debug`, `info`, `warn` or `fatal`)
  - to : file                                 # `stdout`, `stderr` or `file`
    format: json                              # output format `json` (zap.NewJSONEncoder) or "" (zap.NewConsoleEncoder)
    rotate:                                   # `rotate` supports the same fields and behavior as in `accesslog.rotate` (see above)
      cron: "30 * * * *"
      filename: "/path/to/your/errorlog.txt"
      compress: true

# WebSocket configuration (optional)
websocket:
  origins: [ "http://localhost:8000" ]        # auto-check "Origin" header

# HTTPS configuration (optional)
https:
  certFile: /path/to/your/server-cert.pem
  keyFile: /path/to/your/privatekey.pem
```

### Using SQL

First, import any SQL client that is compatible with the `database/sql` package:

```go
import (
  _ "github.com/go-sql-driver/mysql"   // MySQL client (driver: mysql)
  _ "github.com/lib/pq"                // PostgreSQL client (driver: postgres)
  _ "modernc.org/sqlite"               // Pure Go SQLite (driver: sqlite)
  _ "github.com/mattn/go-sqlite3"      // SQLite3 (driver: sqlite3)
)
```

Then, add your SQL settings to the config file.
For example, if you use `sql.Open("sqlite3", "./foo.db")` in Go, your configuration should look like this:

```yaml
sql:
  driver: "sqlite3"
  dsn: "./foo.db"
```


