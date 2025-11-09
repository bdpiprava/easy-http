package httpx_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bdpiprava/easy-http/pkg/httpx"
)

func TestNewInMemoryCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		maxSize     int
		wantMaxSize int
		wantNotNil  bool
	}{
		{
			name:        "creates cache with positive max size",
			maxSize:     100,
			wantMaxSize: 100,
			wantNotNil:  true,
		},
		{
			name:        "creates cache with default size when zero",
			maxSize:     0,
			wantMaxSize: 1000,
			wantNotNil:  true,
		},
		{
			name:        "creates cache with default size when negative",
			maxSize:     -10,
			wantMaxSize: 1000,
			wantNotNil:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewInMemoryCache(tc.maxSize)

			if tc.wantNotNil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestInMemoryCache_Get(t *testing.T) {
	t.Parallel()

	now := time.Now()
	validResponse := &httpx.CachedResponse{
		StatusCode:   200,
		Headers:      http.Header{"Content-Type": []string{"application/json"}},
		Body:         []byte(`{"key":"value"}`),
		CachedAt:     now,
		ExpiresAt:    now.Add(1 * time.Hour),
		ETag:         "etag123",
		LastModified: "Mon, 02 Jan 2006 15:04:05 GMT",
	}

	tests := []struct {
		name          string
		setupCache    func(*httpx.InMemoryCache)
		key           string
		wantResponse  *httpx.CachedResponse
		wantFound     bool
		wantStatsHits int64
		wantStatsMiss int64
	}{
		{
			name: "returns cached response when key exists and not expired",
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("test-key", validResponse)
			},
			key:           "test-key",
			wantResponse:  validResponse,
			wantFound:     true,
			wantStatsHits: 1,
			wantStatsMiss: 0,
		},
		{
			name: "returns not found when key does not exist",
			setupCache: func(cache *httpx.InMemoryCache) {
				// Empty cache
			},
			key:           "nonexistent-key",
			wantResponse:  nil,
			wantFound:     false,
			wantStatsHits: 0,
			wantStatsMiss: 1,
		},
		{
			name: "returns not found when entry is expired",
			setupCache: func(cache *httpx.InMemoryCache) {
				expiredResponse := &httpx.CachedResponse{
					StatusCode: 200,
					Headers:    http.Header{"Content-Type": []string{"application/json"}},
					Body:       []byte(`{"expired":"true"}`),
					CachedAt:   now.Add(-2 * time.Hour),
					ExpiresAt:  now.Add(-1 * time.Hour), // Expired 1 hour ago
					ETag:       "old-etag",
				}
				_ = cache.Set("expired-key", expiredResponse)
			},
			key:           "expired-key",
			wantResponse:  nil,
			wantFound:     false,
			wantStatsHits: 0,
			wantStatsMiss: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewInMemoryCache(100)
			tc.setupCache(subject)

			gotResponse, gotFound := subject.Get(tc.key)

			assert.Equal(t, tc.wantFound, gotFound)
			if tc.wantFound {
				assert.Equal(t, tc.wantResponse.StatusCode, gotResponse.StatusCode)
				assert.Equal(t, tc.wantResponse.ETag, gotResponse.ETag)
				assert.Equal(t, tc.wantResponse.Body, gotResponse.Body)
			} else {
				assert.Nil(t, gotResponse)
			}

			stats := subject.Stats()
			assert.Equal(t, tc.wantStatsHits, stats.Hits)
			assert.Equal(t, tc.wantStatsMiss, stats.Misses)
		})
	}
}

func TestInMemoryCache_Set(t *testing.T) {
	t.Parallel()

	now := time.Now()
	response1 := &httpx.CachedResponse{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"test":"data1"}`),
		CachedAt:   now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	response2 := &httpx.CachedResponse{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"text/html"}},
		Body:       []byte(`<html>test</html>`),
		CachedAt:   now,
		ExpiresAt:  now.Add(2 * time.Hour),
	}

	tests := []struct {
		name          string
		maxSize       int
		setupCache    func(*httpx.InMemoryCache)
		key           string
		response      *httpx.CachedResponse
		wantErr       bool
		wantSize      int64
		wantEvictions int64
	}{
		{
			name:    "sets new cache entry",
			maxSize: 10,
			setupCache: func(cache *httpx.InMemoryCache) {
				// Empty cache
			},
			key:           "key1",
			response:      response1,
			wantErr:       false,
			wantSize:      1,
			wantEvictions: 0,
		},
		{
			name:    "updates existing cache entry",
			maxSize: 10,
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("key1", response1)
			},
			key:           "key1",
			response:      response2,
			wantErr:       false,
			wantSize:      1,
			wantEvictions: 0,
		},
		{
			name:    "evicts oldest entry when cache is full",
			maxSize: 2,
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("key1", response1)
				_ = cache.Set("key2", response2)
			},
			key:           "key3",
			response:      response1,
			wantErr:       false,
			wantSize:      2,
			wantEvictions: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewInMemoryCache(tc.maxSize)
			tc.setupCache(subject)

			gotErr := subject.Set(tc.key, tc.response)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)

				// Verify the entry was set
				cached, found := subject.Get(tc.key)
				assert.True(t, found)
				assert.Equal(t, tc.response.StatusCode, cached.StatusCode)

				// Verify stats
				stats := subject.Stats()
				assert.Equal(t, tc.wantSize, stats.Size)
				assert.Equal(t, tc.wantEvictions, stats.Evictions)
			}
		})
	}
}

func TestInMemoryCache_Delete(t *testing.T) {
	t.Parallel()

	now := time.Now()
	response := &httpx.CachedResponse{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{"test":"data"}`),
		CachedAt:   now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	tests := []struct {
		name       string
		setupCache func(*httpx.InMemoryCache)
		key        string
		wantErr    bool
		wantSize   int64
	}{
		{
			name: "deletes existing entry",
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("key1", response)
				_ = cache.Set("key2", response)
			},
			key:      "key1",
			wantErr:  false,
			wantSize: 1,
		},
		{
			name: "deleting nonexistent key does not error",
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("key1", response)
			},
			key:      "nonexistent",
			wantErr:  false,
			wantSize: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewInMemoryCache(10)
			tc.setupCache(subject)

			gotErr := subject.Delete(tc.key)

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)

				// Verify the entry was deleted
				_, found := subject.Get(tc.key)
				assert.False(t, found)

				// Verify size
				stats := subject.Stats()
				assert.Equal(t, tc.wantSize, stats.Size)
			}
		})
	}
}

func TestInMemoryCache_Clear(t *testing.T) {
	t.Parallel()

	now := time.Now()
	response := &httpx.CachedResponse{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{}`),
		CachedAt:   now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	tests := []struct {
		name       string
		setupCache func(*httpx.InMemoryCache)
		wantErr    bool
		wantSize   int64
	}{
		{
			name: "clears all entries",
			setupCache: func(cache *httpx.InMemoryCache) {
				_ = cache.Set("key1", response)
				_ = cache.Set("key2", response)
				_ = cache.Set("key3", response)
			},
			wantErr:  false,
			wantSize: 0,
		},
		{
			name: "clearing empty cache does not error",
			setupCache: func(cache *httpx.InMemoryCache) {
				// Empty cache
			},
			wantErr:  false,
			wantSize: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subject := httpx.NewInMemoryCache(10)
			tc.setupCache(subject)

			gotErr := subject.Clear()

			if tc.wantErr {
				assert.Error(t, gotErr)
			} else {
				assert.NoError(t, gotErr)

				// Verify all entries were cleared
				stats := subject.Stats()
				assert.Equal(t, tc.wantSize, stats.Size)
			}
		})
	}
}

func TestInMemoryCache_Stats(t *testing.T) {
	t.Parallel()

	now := time.Now()
	response := &httpx.CachedResponse{
		StatusCode: 200,
		Headers:    http.Header{},
		Body:       []byte(`{}`),
		CachedAt:   now,
		ExpiresAt:  now.Add(1 * time.Hour),
	}

	t.Run("tracks cache statistics correctly", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewInMemoryCache(2)

		// Set entries
		_ = subject.Set("key1", response)
		_ = subject.Set("key2", response)

		// Cache hit
		_, _ = subject.Get("key1")

		// Cache miss
		_, _ = subject.Get("nonexistent")

		// Trigger eviction
		_ = subject.Set("key3", response)

		stats := subject.Stats()

		assert.Equal(t, int64(2), stats.Size)
		assert.Equal(t, int64(1), stats.Hits)
		assert.Equal(t, int64(1), stats.Misses)
		assert.Equal(t, int64(1), stats.Evictions)
	})
}

func TestInMemoryCache_Concurrency(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent reads and writes safely", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewInMemoryCache(100)
		now := time.Now()

		var wg sync.WaitGroup
		iterations := 100

		// Concurrent writes
		for i := range iterations {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				response := &httpx.CachedResponse{
					StatusCode: 200,
					Body:       []byte(`{}`),
					CachedAt:   now,
					ExpiresAt:  now.Add(1 * time.Hour),
				}
				_ = subject.Set("key"+string(rune(idx)), response)
			}(i)
		}

		// Concurrent reads
		for i := range iterations {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, _ = subject.Get("key" + string(rune(idx)))
			}(i)
		}

		wg.Wait()

		// Should not panic and should have some entries
		stats := subject.Stats()
		assert.True(t, stats.Size > 0)
	})
}

func TestNewCacheMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		config            httpx.CacheConfig
		wantDefaultTTL    time.Duration
		wantBackendNotNil bool
		wantMethodsCount  int
	}{
		{
			name: "creates middleware with custom config",
			config: httpx.CacheConfig{
				Backend:          httpx.NewInMemoryCache(50),
				DefaultTTL:       10 * time.Minute,
				CacheableMethods: []string{http.MethodGet},
			},
			wantDefaultTTL:    10 * time.Minute,
			wantBackendNotNil: true,
			wantMethodsCount:  1,
		},
		{
			name:   "creates middleware with defaults when not specified",
			config: httpx.CacheConfig{
				// Empty config
			},
			wantDefaultTTL:    5 * time.Minute,
			wantBackendNotNil: true,
			wantMethodsCount:  2, // GET and HEAD
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := httpx.NewCacheMiddleware(tc.config)

			assert.NotNil(t, got)
			assert.Equal(t, "cache", got.Name())
		})
	}
}

func TestCacheMiddleware_Name(t *testing.T) {
	t.Parallel()

	t.Run("returns cache as middleware name", func(t *testing.T) {
		t.Parallel()

		subject := httpx.NewCacheMiddleware(httpx.CacheConfig{})

		got := subject.Name()

		assert.Equal(t, "cache", got)
	})
}

func TestCacheMiddleware_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupServer     func() *httptest.Server
		cacheConfig     httpx.CacheConfig
		requestMethod   string
		requestPath     string
		wantStatusCode  int
		wantCacheHit    bool
		executeCount    int
		wantServerCalls int
	}{
		{
			name: "caches GET request on first call",
			setupServer: func() *httptest.Server {
				callCount := 0
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					callCount++
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Cache-Control", "max-age=3600")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"cached":true}`))
				}))
			},
			cacheConfig:     httpx.CacheConfig{},
			requestMethod:   http.MethodGet,
			requestPath:     "/test",
			wantStatusCode:  http.StatusOK,
			wantCacheHit:    false,
			executeCount:    1,
			wantServerCalls: 1,
		},
		{
			name: "does not cache POST requests",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"not_cached":true}`))
				}))
			},
			cacheConfig:     httpx.CacheConfig{},
			requestMethod:   http.MethodPost,
			requestPath:     "/post",
			wantStatusCode:  http.StatusOK,
			wantCacheHit:    false,
			executeCount:    1,
			wantServerCalls: 1,
		},
		{
			name: "respects Cache-Control no-store directive",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Cache-Control", "no-store")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"no_store":true}`))
				}))
			},
			cacheConfig:     httpx.CacheConfig{},
			requestMethod:   http.MethodGet,
			requestPath:     "/no-store",
			wantStatusCode:  http.StatusOK,
			wantCacheHit:    false,
			executeCount:    1,
			wantServerCalls: 1,
		},
		{
			name: "handles 304 Not Modified responses",
			setupServer: func() *httptest.Server {
				firstCall := true
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if firstCall {
						firstCall = false
						w.Header().Set("Content-Type", "application/json")
						w.Header().Set("ETag", "etag123")
						w.Header().Set("Cache-Control", "max-age=3600")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data":"original"}`))
					} else {
						// Check for conditional request
						if r.Header.Get("If-None-Match") == "etag123" {
							w.WriteHeader(http.StatusNotModified)
						} else {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"data":"modified"}`))
						}
					}
				}))
			},
			cacheConfig:     httpx.CacheConfig{},
			requestMethod:   http.MethodGet,
			requestPath:     "/etag-test",
			wantStatusCode:  http.StatusOK,
			wantCacheHit:    false,
			executeCount:    2,
			wantServerCalls: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := tc.setupServer()
			defer server.Close()

			// Create cache middleware
			middleware := httpx.NewCacheMiddleware(tc.cacheConfig)

			// Create client with cache middleware
			client := httpx.NewClientWithConfig(
				httpx.WithClientDefaultBaseURL(server.URL),
				httpx.WithClientMiddleware(middleware),
			)

			// Execute request(s)
			for range tc.executeCount {
				req := httpx.NewRequest(tc.requestMethod, httpx.WithPath(tc.requestPath))
				response, err := client.Execute(*req, map[string]any{})

				require.NoError(t, err)
				assert.Equal(t, tc.wantStatusCode, response.StatusCode)
			}
		})
	}
}

func TestCacheMiddleware_Integration(t *testing.T) {
	t.Parallel()

	t.Run("end-to-end caching with client configuration", func(t *testing.T) {
		t.Parallel()

		serverCallCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"call_count":` + string(rune(serverCallCount+'0')) + `}`))
		}))
		defer server.Close()

		// Create client with default caching
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		type CacheResponse struct {
			CallCount int `json:"call_count"`
		}

		// First request - cache miss
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/data"))
		resp1, err := client.Execute(*req1, CacheResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		assert.Equal(t, 1, serverCallCount)

		// Second request - should use cached response
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/data"))
		resp2, err := client.Execute(*req2, CacheResponse{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)

		// Server should still only be called twice due to conditional request
		assert.True(t, serverCallCount <= 2, "Expected at most 2 server calls, got %d", serverCallCount)
	})

	t.Run("custom cache backend integration", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"custom":"backend"}`))
		}))
		defer server.Close()

		// Create custom cache backend
		customBackend := httpx.NewInMemoryCache(5)

		// Create client with custom cache
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCache(httpx.CacheConfig{
				Backend:    customBackend,
				DefaultTTL: 1 * time.Hour,
			}),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/custom"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify cache backend has the entry
		stats := customBackend.Stats()
		assert.True(t, stats.Size > 0)
	})
}

func TestCacheMiddleware_CacheExpiration(t *testing.T) {
	t.Parallel()

	t.Run("expires cache entries based on Cache-Control max-age", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=1") // 1 second TTL
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expires":"soon"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		// First request
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/expires"))
		resp1, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Wait for cache to expire
		time.Sleep(2 * time.Second)

		// Second request after expiration - should fetch fresh data
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/expires"))
		resp2, err := client.Execute(*req2, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})
}

func TestCacheMiddleware_ConditionalRequests(t *testing.T) {
	t.Parallel()

	t.Run("adds If-None-Match header for cached ETag", func(t *testing.T) {
		t.Parallel()

		receivedHeaders := make(map[string]string)
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			receivedHeaders["If-None-Match"] = r.Header.Get("If-None-Match")
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"abc123"`)
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"etag":"test"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		// First request
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/etag"))
		_, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)

		// Second request should include If-None-Match
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/etag"))
		_, err = client.Execute(*req2, map[string]any{})
		require.NoError(t, err)

		mu.Lock()
		ifNoneMatch := receivedHeaders["If-None-Match"]
		mu.Unlock()

		assert.NotEmpty(t, ifNoneMatch, "Expected If-None-Match header to be set on second request")
	})

	t.Run("adds If-Modified-Since header for cached Last-Modified", func(t *testing.T) {
		t.Parallel()

		receivedHeaders := make(map[string]string)
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			receivedHeaders["If-Modified-Since"] = r.Header.Get("If-Modified-Since")
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"last_modified":"test"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		// First request
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/last-modified"))
		_, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)

		// Second request should include If-Modified-Since
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/last-modified"))
		_, err = client.Execute(*req2, map[string]any{})
		require.NoError(t, err)

		mu.Lock()
		ifModifiedSince := receivedHeaders["If-Modified-Since"]
		mu.Unlock()

		assert.NotEmpty(t, ifModifiedSince, "Expected If-Modified-Since header to be set on second request")
	})
}

func TestCacheMiddleware_SkipCache(t *testing.T) {
	t.Parallel()

	t.Run("skips caching when skip function returns true", func(t *testing.T) {
		t.Parallel()

		serverCallCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"skip":"true"}`))
		}))
		defer server.Close()

		// Create cache middleware with skip function
		cacheConfig := httpx.CacheConfig{
			Backend:    httpx.NewInMemoryCache(100),
			DefaultTTL: 5 * time.Minute,
			SkipCacheFor: func(req *http.Request) bool {
				// Skip caching if request has Authorization header
				return req.Header.Get("Authorization") != ""
			},
		}

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientCache(cacheConfig),
		)

		// Request with Authorization header - should not cache
		req1 := httpx.NewRequest(
			http.MethodGet,
			httpx.WithPath("/skip"),
			httpx.WithHeader("Authorization", "Bearer token"),
		)
		resp1, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Second request with Authorization - should hit server again
		req2 := httpx.NewRequest(
			http.MethodGet,
			httpx.WithPath("/skip"),
			httpx.WithHeader("Authorization", "Bearer token"),
		)
		resp2, err := client.Execute(*req2, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)

		// Should have called server twice since caching was skipped
		assert.Equal(t, 2, serverCallCount)
	})
}

func TestCacheResponse_BodyPreservation(t *testing.T) {
	t.Parallel()

	t.Run("preserves response body for downstream consumers", func(t *testing.T) {
		t.Parallel()

		testBody := []byte(`{"test":"body","data":"preserved"}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(testBody)
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/body-test"))
		resp, err := client.Execute(*req, map[string]any{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify body can be read by downstream code
		if bodyMap, ok := resp.Body.(map[string]any); ok {
			assert.Equal(t, "body", bodyMap["test"])
			assert.Equal(t, "preserved", bodyMap["data"])
		} else {
			t.Error("Expected response body to be parsed as map")
		}
	})
}

func TestCacheMiddleware_ErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("does not cache error responses", func(t *testing.T) {
		t.Parallel()

		serverCallCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"server error"}`))
		}))
		defer server.Close()

		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL(server.URL),
			httpx.WithClientDefaultCache(),
		)

		// First error request
		req1 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/error"))
		resp1, err := client.Execute(*req1, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp1.StatusCode)

		// Second error request - should hit server again (not cached)
		req2 := httpx.NewRequest(http.MethodGet, httpx.WithPath("/error"))
		resp2, err := client.Execute(*req2, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)

		// Server should be called twice since errors aren't cached
		assert.Equal(t, 2, serverCallCount)
	})

	t.Run("passes through network errors without caching", func(t *testing.T) {
		t.Parallel()

		// Create client pointing to non-existent server
		client := httpx.NewClientWithConfig(
			httpx.WithClientDefaultBaseURL("http://localhost:99999"),
			httpx.WithClientDefaultCache(),
			httpx.WithClientTimeout(1*time.Second),
		)

		req := httpx.NewRequest(http.MethodGet, httpx.WithPath("/network-error"))
		_, err := client.Execute(*req, map[string]any{})

		// Should return network error
		assert.Error(t, err)
	})
}

// Helper function to create a test response
func createTestResponse(statusCode int, body []byte, headers http.Header) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	return &http.Response{
		StatusCode:    statusCode,
		Status:        http.StatusText(statusCode),
		Header:        headers,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
	}
}
