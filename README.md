# easy-http

Simple Go utilities and examples for HTTP interactions.

## Features

• Go functions to simplify requests, responses, and testing.  
• Coverage reporting supported via Makefile targets.  
• Uses standard libraries and helpful third-party packages (e.g., YAML, testify).

## Getting Started

1. Clone or download this repo.  
2. Run "make deps" to install dependencies.  
3. Build with "make build" or test with "make tests".

## Usage

### Installation

```bash
go get github.com/bdpiprava/easy-http
```

### Examples

```go
import (
    "fmt"
    "net/http"
    "time"

    "github.com/bdpiprava/easy-http/pkg/httpx"
)

func Example() {
    // Create a client with custom base URL and timeout
    client := httpx.NewClient(
        httpx.WithDefaultBaseURL("https://api.example.com"),
        httpx.WithDefaultTimeout(5 * time.Second),
    )

    // Build a request with a specified path and query parameters
    req := httpx.NewRequest(http.MethodGet,
        httpx.WithPath("v1/users"),
        httpx.WithQueryParam("active", "true"),
    )

    // Execute the request and handle the response
    resp, err := client.Execute(*req, map[string]any{})
    if err != nil {
        // handle error
    }
    fmt.Printf("Status Code: %d\n", resp.StatusCode)
    fmt.Printf("Response Body: %+v\n", resp.Body)
}
```
---

```go
import (
    "fmt"
    "net/http"
    "time"

    "github.com/bdpiprava/easy-http/pkg/httpx"
)

// Perform a GET request with a specified type parameter
func ExampleGET() {
    resp, err := httpx.GET[map[string]any](
        httpx.WithBaseURL("https://api.example.com"),
        httpx.WithPath("v1/posts"),
    )
    if err != nil {
        // handle error
    }
    fmt.Println(resp.Body)
}

// Perform a POST request with a specified type parameter
func ExamplePOST() {
    resp, err := httpx.POST[map[string]any](
        httpx.WithBaseURL("https://api.example.com"),
        httpx.WithPath("v1/posts"),
        httpx.WithHeader("Content-Type", "application/json"),
    )
    if err != nil {
        // handle error
    }
    fmt.Println(resp.Body)
}
```

## Contributing

Open issues or pull requests for improvements or bug fixes.

## License

- [Apache License 2.0](./LICENSE)
