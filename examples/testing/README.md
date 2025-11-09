# Testing Utilities Example

This example demonstrates the comprehensive testing utilities provided by the `httpx/testing` package.

## Overview

The `httpx/testing` package provides a powerful MockServer for testing HTTP clients and services with:

- **Fluent API** for configuring mock responses
- **Request Matchers** for flexible routing
- **Request Recording** for verification
- **Assertions** for easy testing
- **Error Simulation** for testing failure scenarios
- **Flaky Behavior** for testing resilience

## Running the Example

```bash
cd examples/testing
go run main.go
```

## What's Demonstrated

### 1. Basic Mock Server

Shows how to create a mock server and configure simple responses:

```go
mock := httpxtesting.NewMockServer()
defer mock.Close()

mock.OnGet("/users").
    WithStatus(http.StatusOK).
    WithJSON(map[string]interface{}{
        "users": []map[string]interface{}{
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"},
        },
    })
```

### 2. Request Matchers

Demonstrates various ways to match and route requests:

- **Exact Path**: Match specific paths
- **Path Prefix**: Match path prefixes
- **Path Regex**: Match using regular expressions
- **Query Parameters**: Match based on query params
- **Headers**: Match based on request headers
- **Composite**: Combine matchers with AND/OR/NOT logic

### 3. Assertions

Shows how to verify requests were made correctly:

```go
assert := mock.Assert()
assert.RequestCount(2)
assert.RequestTo("/api/data")
assert.RequestWithMethod("POST")
assert.RequestWithHeader("X-Custom", "test-value")
assert.VerifySequence("/api/data", "/api/data")
```

### 4. Error Simulation

Demonstrates simulating various HTTP errors:

- 400 Bad Request
- 401 Unauthorized
- 403 Forbidden
- 404 Not Found
- 429 Too Many Requests
- 500 Internal Server Error
- 503 Service Unavailable
- 504 Gateway Timeout
- Slow responses

### 5. Flaky Behavior

Shows how to test resilience by simulating intermittent failures:

```go
mock.OnFlaky("/flaky", 2).
    WithPattern(3, 2).  // Succeed 3 times, fail 2 times, repeat
    Configure()
```

## Using in Your Tests

Here's a typical test structure:

```go
func TestMyHTTPClient(t *testing.T) {
    // Create mock server
    mock := httpxtesting.NewMockServer()
    defer mock.Close()
    
    // Configure expected responses
    mock.OnGet("/api/data").
        WithStatus(http.StatusOK).
        WithJSON(map[string]interface{}{
            "result": "success",
        })
    
    // Test your client using mock.URL()
    client := NewMyClient(mock.URL())
    result, err := client.GetData()
    
    // Verify behavior
    assert.NoError(t, err)
    assert.Equal(t, "success", result)
    
    // Verify requests
    assert.NoError(t, mock.Assert().RequestCount(1))
    assert.NoError(t, mock.Assert().RequestTo("/api/data"))
}
```

## Key Features

- **Thread-Safe**: All operations are safe for concurrent use
- **No External Dependencies**: Uses only standard library (except testify for tests)
- **Realistic**: Built on `httptest.Server` for accurate HTTP behavior
- **Flexible**: Supports any HTTP method, header, status code
- **Comprehensive**: 190+ tests ensuring reliability

## See Also

- [MockServer API Documentation](../../pkg/httpx/testing/mock_server.go)
- [Assertions Documentation](../../pkg/httpx/testing/assertions.go)
- [Error Simulation Documentation](../../pkg/httpx/testing/errors.go)
- [Full Test Suite](../../pkg/httpx/testing/)
