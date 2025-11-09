package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CacheBackend defines the interface for cache storage
type CacheBackend interface {
	Get(key string) (*CachedResponse, bool)
	Set(key string, response *CachedResponse) error
	Delete(key string) error
	Clear() error
	Stats() CacheStats
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode   int
	Headers      http.Header
	Body         []byte
	CachedAt     time.Time
	ExpiresAt    time.Time
	ETag         string
	LastModified string
}

// CacheConfig configures the caching middleware
type CacheConfig struct {
	Backend          CacheBackend
	MaxEntries       int
	MaxSizeBytes     int64
	DefaultTTL       time.Duration
	CacheableMethods []string
	SkipCacheFor     func(*http.Request) bool
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int64
}

// InMemoryCache implements CacheBackend using an in-memory store
type InMemoryCache struct {
	mu       sync.RWMutex
	entries  map[string]*CachedResponse
	stats    CacheStats
	maxSize  int
	lruOrder []string // Track access order for LRU eviction
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxSize int) *InMemoryCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default size
	}
	return &InMemoryCache{
		entries:  make(map[string]*CachedResponse),
		maxSize:  maxSize,
		lruOrder: make([]string, 0, maxSize),
	}
}

// Get retrieves a cached response
func (c *InMemoryCache) Get(key string) (*CachedResponse, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.removeFromLRU(key)
		c.stats.Misses++
		c.stats.Evictions++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.stats.Hits++
	c.updateLRU(key)
	c.mu.Unlock()

	return entry, true
}

// Set stores a response in cache
func (c *InMemoryCache) Set(key string, response *CachedResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize && c.entries[key] == nil {
		c.evictOldest()
	}

	c.entries[key] = response
	c.updateLRU(key)
	c.stats.Size = int64(len(c.entries))
	return nil
}

// Delete removes a cache entry
func (c *InMemoryCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
	c.removeFromLRU(key)
	c.stats.Size = int64(len(c.entries))
	return nil
}

// Clear removes all cache entries
func (c *InMemoryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CachedResponse)
	c.lruOrder = make([]string, 0, c.maxSize)
	c.stats.Size = 0
	return nil
}

// Stats returns cache statistics
func (c *InMemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// evictOldest removes the least recently used entry
func (c *InMemoryCache) evictOldest() {
	if len(c.lruOrder) == 0 {
		return
	}

	oldest := c.lruOrder[0]
	delete(c.entries, oldest)
	c.lruOrder = c.lruOrder[1:]
	c.stats.Evictions++
}

// updateLRU moves a key to the end of the LRU list
func (c *InMemoryCache) updateLRU(key string) {
	// Remove from current position
	c.removeFromLRU(key)
	// Add to end
	c.lruOrder = append(c.lruOrder, key)
}

// removeFromLRU removes a key from the LRU list
func (c *InMemoryCache) removeFromLRU(key string) {
	for i, k := range c.lruOrder {
		if k == key {
			c.lruOrder = append(c.lruOrder[:i], c.lruOrder[i+1:]...)
			break
		}
	}
}

// CacheMiddleware implements HTTP caching
type CacheMiddleware struct {
	config CacheConfig
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(config CacheConfig) *CacheMiddleware {
	if config.Backend == nil {
		config.Backend = NewInMemoryCache(1000)
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 5 * time.Minute
	}
	if len(config.CacheableMethods) == 0 {
		config.CacheableMethods = []string{http.MethodGet, http.MethodHead}
	}
	return &CacheMiddleware{config: config}
}

// Name returns the middleware name
func (m *CacheMiddleware) Name() string {
	return "cache"
}

// Execute implements the Middleware interface
func (m *CacheMiddleware) Execute(ctx context.Context, req *http.Request, next MiddlewareFunc) (*http.Response, error) {
	// Check if request is cacheable
	if !m.isCacheable(req) {
		return next(ctx, req)
	}

	cacheKey := m.generateCacheKey(req)

	// Try to get from cache
	if cached, found := m.config.Backend.Get(cacheKey); found {
		// Add conditional request headers
		if cached.ETag != "" {
			req.Header.Set("If-None-Match", cached.ETag)
		}
		if cached.LastModified != "" {
			req.Header.Set("If-Modified-Since", cached.LastModified)
		}
	}

	// Execute request
	resp, err := next(ctx, req)
	if err != nil {
		return nil, err
	}

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		if cached, found := m.config.Backend.Get(cacheKey); found {
			return m.buildResponseFromCache(cached), nil
		}
	}

	// Cache successful responses
	if m.shouldCache(resp) {
		if err := m.cacheResponse(cacheKey, resp); err != nil {
			// Log error but don't fail the request
			// In production, you might want to log this
		}
	}

	return resp, nil
}

// isCacheable checks if the request should use caching
func (m *CacheMiddleware) isCacheable(req *http.Request) bool {
	// Check if method is cacheable
	cacheable := false
	for _, method := range m.config.CacheableMethods {
		if req.Method == method {
			cacheable = true
			break
		}
	}

	if !cacheable {
		return false
	}

	// Check custom skip function
	if m.config.SkipCacheFor != nil && m.config.SkipCacheFor(req) {
		return false
	}

	return true
}

// generateCacheKey creates a unique cache key for the request
func (m *CacheMiddleware) generateCacheKey(req *http.Request) string {
	// Basic key: method + URL
	key := fmt.Sprintf("%s:%s", req.Method, req.URL.String())

	// Add Vary header consideration if present in request context
	// Note: Full Vary support would require storing response headers first
	return key
}

// shouldCache determines if response should be cached
func (m *CacheMiddleware) shouldCache(resp *http.Response) bool {
	// Check Cache-Control directives
	cacheControl := resp.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "no-cache") {
		return false
	}

	// Only cache successful responses
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// cacheResponse stores a response in the cache
func (m *CacheMiddleware) cacheResponse(key string, resp *http.Response) error {
	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Restore body for downstream consumers
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Calculate expiration time
	expiresAt := m.calculateExpiration(resp)

	// Create cached response
	cached := &CachedResponse{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header.Clone(),
		Body:         bodyBytes,
		CachedAt:     time.Now(),
		ExpiresAt:    expiresAt,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	return m.config.Backend.Set(key, cached)
}

// calculateExpiration determines when a cached response expires
func (m *CacheMiddleware) calculateExpiration(resp *http.Response) time.Time {
	// Check Cache-Control max-age
	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl != "" {
		directives := strings.Split(cacheControl, ",")
		for _, directive := range directives {
			directive = strings.TrimSpace(directive)
			if strings.HasPrefix(directive, "max-age=") {
				maxAgeStr := strings.TrimPrefix(directive, "max-age=")
				var maxAge int
				if _, err := fmt.Sscanf(maxAgeStr, "%d", &maxAge); err == nil && maxAge > 0 {
					return time.Now().Add(time.Duration(maxAge) * time.Second)
				}
			}
		}
	}

	// Check Expires header
	if expiresStr := resp.Header.Get("Expires"); expiresStr != "" {
		if expiresTime, err := http.ParseTime(expiresStr); err == nil {
			return expiresTime
		}
	}

	// Use default TTL
	return time.Now().Add(m.config.DefaultTTL)
}

// buildResponseFromCache reconstructs an HTTP response from cache
func (m *CacheMiddleware) buildResponseFromCache(cached *CachedResponse) *http.Response {
	return &http.Response{
		StatusCode:    cached.StatusCode,
		Status:        http.StatusText(cached.StatusCode),
		Header:        cached.Headers.Clone(),
		Body:          io.NopCloser(bytes.NewReader(cached.Body)),
		ContentLength: int64(len(cached.Body)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
	}
}
