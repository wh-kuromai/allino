# allino - Old README

This file keeps the older README tone and concept notes as a historical record.
Practical setup, typed function usage, MCP, jobs, CLI, auth, and the AI prompt
template have moved back into the current [README](../README.md) and dedicated
docs.

---

# allino

[![Go Report Card](https://goreportcard.com/badge/github.com/wh-kuromai/allino)](https://goreportcard.com/report/github.com/wh-kuromai/allino)
[![Go Reference](https://pkg.go.dev/badge/github.com/wh-kuromai/allino.svg)](https://pkg.go.dev/github.com/wh-kuromai/allino)

**AI-first web framework for Go**
Let your AI generate your apps using best-practice OSS – automatically.

---

## AI Result

Examples of AI-generated results using the prepared AI Prompt Template from the
old README.

| Description | Type | AI | Result |
| --- | --- | --- | --- |
| Create simple QR code API | api, binary | ChatGPT-4o | [Result](./result/qrcode.md) |
| Create simple short URL API | api, redis, path-param, redirect | ChatGPT-4o | [Result](./result/shorturl.md) |
| Create simple ID Registration and ID/Password login API | 2-apis, redis, sql, login, cookie | ChatGPT-4o | [1](./result/idpw_login.md), [2](./result/idpw_login.md) |

---

## Benchmark

| Software | githubAPI | | gplusAPI | | parseAPI | |
|----------|-----------------|--------|-----------------|--------|-----------------|--------|
| fiber    | 101,496 req/sec | 1.06ms | 112,741 req/sec | 0.96ms | 108,303 req/sec | 0.99ms |
| allino (fiber +validation,etc)   |  86,738 req/sec | 1.30ms |  94,450 req/sec | 1.22ms |  95,382 req/sec | 1.18ms |
| echo     |  84,966 req/sec | 1.33ms |  80,391 req/sec | 1.42ms |  79,833 req/sec | 1.44ms |
| gin      |  84,373 req/sec | 1.37ms |  83,669 req/sec | 1.36ms |  83,033 req/sec | 1.45ms |

use test-data from https://github.com/vishr/web-framework-benchmark

---

## Inspiration

I created this framework with questions like:

- How can we get AI to write code that uses fiber, zap, and go-redis properly?
- How should input/output be described for AI?
- How can we keep the generated code compact?
- How do we describe previously generated code to AI?

Most existing frameworks prioritize human readability and flexibility. But for
AI, that often makes it unclear which tools are allowed. For example, when AI
needs to use `go-redis`, it might start by opening a database connection inside
the handler. If you say "use `zap`", the AI might begin by writing `zap`
initialization code. And when it needs a user ID, it may generate a fake
placeholder instead of using proper authentication.

allino solves this by encoding those expectations into the framework itself.

Once you've built a few APIs, try running

```bash
go run main.go gendoc openapi
```

and giving the result to an AI. It will likely respond:

> This is great! Maybe we can also add an endpoint like X?

That is when things get really fun.
And the best part? The AI will thank *you* for making its life easier.
