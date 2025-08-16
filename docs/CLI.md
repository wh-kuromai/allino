
# allino - CLI

### ✍ allino cli output sample

```sh
~/github/allino/example/simple main*
❯ go run main.go
allino - AI-first web framework server

Usage:
  allino [command]

Available Commands:
  completion   Generate the autocompletion script for the specified shell
  encrypt      Encrypt config file
  help         Help about any command
  keygen       Generate secrets.config.json file
  openapi      Generate OpenAPI YAML
  route        Print registered routes
  serve        Start the web server
  version      Print version info

Flags:
  -b, --bind string         Set HTTP server bind address
  -c, --config-dir string   Set config directory path
  -h, --help                help for allino
  -w, --work-dir string     Set working directory path

Use "allino [command] --help" for more information about a command.

~/github/allino/example/simple main*
❯ go run main.go route
GET /api/healthcheck

~/github/allino/example/simple main*
❯ go run main.go openapi
openapi: 3.1.0
info:
  title: allino
  version: 0.0.1
paths:
  /api/healthcheck:
    get:
      parameters:
      - name: echo
        in: query
        required: true
        schema:
          type: string
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: object
                    properties:
                      echo:
                        type: string
                      startAt:
                        type: string
                      status:
                        type: string

~/github/allino/example/simple main*
❯ go run main.go serve  
╭───────────────────────────────────────╮
│                                       │
│   allino - AI-first web framework     │
│                                       │
│   Running at: http://localhost:8000   │
│                                       │
╰───────────────────────────────────────╯

1.754701337878464e+09   info    server starting {"protocol": "http", "bind": ":8000", "url": "http://localhost:8000", "startAt": 1754701337.87791}

Shutting down server...
1.75470134210884e+09    info    server shutting down
Server gracefully stopped
1.754701342109249e+09   info    server shutdown complete
```
