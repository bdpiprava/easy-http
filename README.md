# easy-http

**A production-ready HTTP client for Go that doesn't make you cry.**

Stop wrestling with `http.DefaultClient` and endless boilerplate. Get type-safe requests, automatic retries, circuit breakers, observability, and everything else you wish the standard library had—without the complexity.

```go
// That's it. Seriously.
resp, err := httpx.GET[User](
    httpx.WithBaseURL("https://api.example.com"),
    httpx.WithPath("/users/123"),
)
```

**Built for production** with retries, circuit breakers, rate limiting, caching, tracing, metrics, and middleware—all optional, all composable, none of the headache.

## Installation

```bash
go get github.com/bdpiprava/easy-http
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/bdpiprava/easy-http/pkg/httpx"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    // GET with automatic JSON unmarshaling
    user, err := httpx.GET[User](
        httpx.WithBaseURL("https://api.example.com"),
        httpx.WithPath("/users/1"),
    )
    if err != nil {
        panic(err)
    }

    fmt.Printf("Hello, %s!\n", user.Body.Name)

    // POST with type safety
    newUser := User{Name: "Alice", Email: "alice@example.com"}
    resp, err := httpx.POST[User](
        httpx.WithBaseURL("https://api.example.com"),
        httpx.WithPath("/users"),
        httpx.WithJSONBody(newUser),
    )

    fmt.Printf("Created user ID: %d\n", resp.Body.ID)
}
```

## What You Get

**Core Features:**
- 🎯 Type-safe generic responses with automatic JSON unmarshaling
- 🔄 Fluent API with functional options (no builders, no bloat)
- 🛡️ Built-in retry logic and circuit breakers
- ⚡ Rate limiting and request throttling
- 🗄️ RFC 7234 compliant HTTP caching
- 📦 Automatic compression (gzip/deflate)

**Observability:**
- 📊 Prometheus metrics out of the box
- 🔍 OpenTelemetry distributed tracing
- 🍪 Smart cookie management
- 🔐 Built-in auth helpers and middleware support

**No magic. No surprises. Just HTTP that works.**

## Documentation

👉 **[Full documentation, guides, and examples](https://bdpiprava.github.io/easy-http)**

## License

[Apache License 2.0](./LICENSE)

---

<div align="center">

**Made with ❤️ for the Go community**

[⭐ Star this repo](https://github.com/bdpiprava/easy-http) • [📖 Documentation](https://bdpiprava.github.io/easy-http) • [🐛 Report Issues](https://github.com/bdpiprava/easy-http/issues) • [💬 Discussions](https://github.com/bdpiprava/easy-http/discussions)

</div>
