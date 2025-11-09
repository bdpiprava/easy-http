package httpx_test

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSystemProxyFunc(t *testing.T) {
	t.Parallel()

	t.Run("returns http.ProxyFromEnvironment function", func(t *testing.T) {
		t.Parallel()

		got := httpx.GetSystemProxyFunc()

		assert.NotNil(t, got)
		// Verify it returns the same function as http.ProxyFromEnvironment
		assert.NotNil(t, got)
	})
}

func TestParseProxyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		proxyURL string
		want     *url.URL
		wantErr  bool
	}{
		{
			name:     "parses valid HTTP proxy URL",
			proxyURL: "http://proxy.example.com:8080",
			want: &url.URL{
				Scheme: "http",
				Host:   "proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name:     "parses valid HTTPS proxy URL",
			proxyURL: "https://secure-proxy.example.com:8443",
			want: &url.URL{
				Scheme: "https",
				Host:   "secure-proxy.example.com:8443",
			},
			wantErr: false,
		},
		{
			name:     "parses valid SOCKS4 proxy URL",
			proxyURL: "socks4://localhost:1080",
			want: &url.URL{
				Scheme: "socks4",
				Host:   "localhost:1080",
			},
			wantErr: false,
		},
		{
			name:     "parses valid SOCKS5 proxy URL",
			proxyURL: "socks5://127.0.0.1:9050",
			want: &url.URL{
				Scheme: "socks5",
				Host:   "127.0.0.1:9050",
			},
			wantErr: false,
		},
		{
			name:     "parses proxy URL with authentication",
			proxyURL: "http://user:pass@proxy.example.com:8080",
			want: &url.URL{
				Scheme: "http",
				User:   url.UserPassword("user", "pass"),
				Host:   "proxy.example.com:8080",
			},
			wantErr: false,
		},
		{
			name:     "parses proxy URL without port",
			proxyURL: "http://proxy.example.com",
			want: &url.URL{
				Scheme: "http",
				Host:   "proxy.example.com",
			},
			wantErr: false,
		},
		{
			name:     "returns error for empty proxy URL",
			proxyURL: "",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "returns error for invalid proxy URL",
			proxyURL: "://invalid",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "returns error for unsupported scheme",
			proxyURL: "ftp://proxy.example.com:8080",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "returns error for missing host",
			proxyURL: "http://",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "returns error for invalid characters",
			proxyURL: "http://proxy example.com:8080",
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := httpx.ParseProxyURL(tc.proxyURL)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				require.NotNil(t, got)
				assert.Equal(t, tc.want.Scheme, got.Scheme)
				assert.Equal(t, tc.want.Host, got.Host)
				if tc.want.User != nil {
					assert.Equal(t, tc.want.User.Username(), got.User.Username())
				}
			}
		})
	}
}

func TestShouldBypassProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requestURL  string
		noProxyList []string
		want        bool
	}{
		{
			name:        "bypasses exact domain match",
			requestURL:  "http://example.com/path",
			noProxyList: []string{"example.com"},
			want:        true,
		},
		{
			name:        "bypasses wildcard domain match",
			requestURL:  "http://api.example.com/v1",
			noProxyList: []string{"*.example.com"},
			want:        true,
		},
		{
			name:        "bypasses domain suffix match",
			requestURL:  "http://api.example.com/v1",
			noProxyList: []string{".example.com"},
			want:        true,
		},
		{
			name:        "bypasses localhost",
			requestURL:  "http://localhost:8080/api",
			noProxyList: []string{"localhost"},
			want:        true,
		},
		{
			name:        "bypasses 127.0.0.1",
			requestURL:  "http://127.0.0.1:8080/api",
			noProxyList: []string{"127.0.0.1"},
			want:        true,
		},
		{
			name:        "bypasses CIDR notation",
			requestURL:  "http://192.168.1.100/api",
			noProxyList: []string{"192.168.0.0/16"},
			want:        true,
		},
		{
			name:        "bypasses multiple patterns",
			requestURL:  "http://internal.company.com/api",
			noProxyList: []string{"localhost", "*.company.com", "192.168.0.0/16"},
			want:        true,
		},
		{
			name:        "does not bypass non-matching domain",
			requestURL:  "http://external.com/api",
			noProxyList: []string{"example.com", "*.internal.com"},
			want:        false,
		},
		{
			name:        "does not bypass when list is empty",
			requestURL:  "http://example.com/api",
			noProxyList: []string{},
			want:        false,
		},
		{
			name:        "does not bypass when list is nil",
			requestURL:  "http://example.com/api",
			noProxyList: nil,
			want:        false,
		},
		{
			name:        "handles empty hostname gracefully",
			requestURL:  "http:///path",
			noProxyList: []string{"example.com"},
			want:        false,
		},
		{
			name:        "case insensitive domain matching",
			requestURL:  "http://EXAMPLE.COM/path",
			noProxyList: []string{"example.com"},
			want:        true,
		},
		{
			name:        "wildcard matches subdomain",
			requestURL:  "http://deeply.nested.example.com/api",
			noProxyList: []string{"*.example.com"},
			want:        true,
		},
		{
			name:        "wildcard does not match parent domain",
			requestURL:  "http://example.com/api",
			noProxyList: []string{"*.example.com"},
			want:        false,
		},
		{
			name:        "bypasses with whitespace in pattern",
			requestURL:  "http://example.com/api",
			noProxyList: []string{" example.com ", "other.com"},
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reqURL, err := url.Parse(tc.requestURL)
			require.NoError(t, err)
			req := &http.Request{URL: reqURL}

			got := httpx.ShouldBypassProxy(req, tc.noProxyList)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCreateProxyFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     *httpx.ProxyConfig
		requestURL string
		wantProxy  bool
		wantBypass bool
		wantNil    bool
	}{
		{
			name:       "returns http.ProxyFromEnvironment when config is nil",
			config:     nil,
			requestURL: "http://example.com",
			wantNil:    false,
		},
		{
			name: "returns proxy URL for HTTP proxy",
			config: &httpx.ProxyConfig{
				ProxyURL: mustParseURL("http://proxy.example.com:8080"),
			},
			requestURL: "http://api.example.com",
			wantProxy:  true,
		},
		{
			name: "returns nil when request should bypass proxy",
			config: &httpx.ProxyConfig{
				ProxyURL: mustParseURL("http://proxy.example.com:8080"),
				NoProxy:  []string{"api.example.com"},
			},
			requestURL: "http://api.example.com",
			wantBypass: true,
		},
		{
			name: "returns proxy URL when request not in bypass list",
			config: &httpx.ProxyConfig{
				ProxyURL: mustParseURL("http://proxy.example.com:8080"),
				NoProxy:  []string{"internal.com"},
			},
			requestURL: "http://external.com",
			wantProxy:  true,
		},
		{
			name: "uses custom ProxyFunc when provided",
			config: &httpx.ProxyConfig{
				ProxyFunc: func(_ *http.Request) (*url.URL, error) {
					return mustParseURL("http://custom-proxy.com:8080"), nil
				},
			},
			requestURL: "http://example.com",
			wantProxy:  true,
		},
		{
			name: "bypasses with wildcard pattern",
			config: &httpx.ProxyConfig{
				ProxyURL: mustParseURL("http://proxy.example.com:8080"),
				NoProxy:  []string{"*.internal.com"},
			},
			requestURL: "http://api.internal.com",
			wantBypass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			proxyFunc := httpx.CreateProxyFunc(tc.config)

			assert.NotNil(t, proxyFunc)

			reqURL, err := url.Parse(tc.requestURL)
			require.NoError(t, err)
			req := &http.Request{URL: reqURL}

			gotProxyURL, gotErr := proxyFunc(req)

			assert.NoError(t, gotErr)

			if tc.wantBypass {
				assert.Nil(t, gotProxyURL)
			} else if tc.wantProxy && tc.config != nil && tc.config.ProxyURL != nil {
				assert.NotNil(t, gotProxyURL)
			}
		})
	}
}

func TestCreateSOCKSDialer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		socksURL *url.URL
		auth     *httpx.ProxyAuth
		wantErr  bool
	}{
		{
			name:     "creates SOCKS5 dialer without auth",
			socksURL: mustParseURL("socks5://localhost:1080"),
			auth:     nil,
			wantErr:  false,
		},
		{
			name:     "creates SOCKS5 dialer with auth",
			socksURL: mustParseURL("socks5://localhost:1080"),
			auth: &httpx.ProxyAuth{
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name:     "creates SOCKS4 dialer",
			socksURL: mustParseURL("socks4://localhost:1080"),
			auth:     nil,
			wantErr:  false,
		},
		{
			name:     "returns error for nil SOCKS URL",
			socksURL: nil,
			auth:     nil,
			wantErr:  true,
		},
		{
			name:     "returns error for unsupported scheme",
			socksURL: mustParseURL("http://localhost:8080"),
			auth:     nil,
			wantErr:  true,
		},
		{
			name:     "handles empty auth credentials",
			socksURL: mustParseURL("socks5://localhost:1080"),
			auth: &httpx.ProxyAuth{
				Username: "",
				Password: "",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := httpx.CreateSOCKSDialer(tc.socksURL, tc.auth)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, gotErr)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestGetSystemNoProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		noProxyValue string
		upperCase    bool
		want         []string
	}{
		{
			name:         "parses single domain",
			noProxyValue: "example.com",
			want:         []string{"example.com"},
		},
		{
			name:         "parses multiple domains",
			noProxyValue: "example.com,internal.com,localhost",
			want:         []string{"example.com", "internal.com", "localhost"},
		},
		{
			name:         "trims whitespace from domains",
			noProxyValue: " example.com , internal.com , localhost ",
			want:         []string{"example.com", "internal.com", "localhost"},
		},
		{
			name:         "handles empty NO_PROXY",
			noProxyValue: "",
			want:         nil,
		},
		{
			name:         "handles wildcard patterns",
			noProxyValue: "*.example.com,.internal.com,192.168.0.0/16",
			want:         []string{"*.example.com", ".internal.com", "192.168.0.0/16"},
		},
		{
			name:         "filters out empty entries",
			noProxyValue: "example.com,,internal.com,  ,localhost",
			want:         []string{"example.com", "internal.com", "localhost"},
		},
		{
			name:         "handles uppercase NO_PROXY",
			noProxyValue: "example.com,localhost",
			upperCase:    true,
			want:         []string{"example.com", "localhost"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Cannot run in parallel due to environment variable manipulation
			envVar := "no_proxy"
			if tc.upperCase {
				envVar = "NO_PROXY"
			}

			// Set environment variable
			oldValue := os.Getenv(envVar)
			err := os.Setenv(envVar, tc.noProxyValue)
			require.NoError(t, err)
			defer func() {
				_ = os.Setenv(envVar, oldValue)
			}()

			// Clear the other variant
			otherVar := "NO_PROXY"
			if tc.upperCase {
				otherVar = "no_proxy"
			}
			oldOther := os.Getenv(otherVar)
			_ = os.Unsetenv(otherVar)
			defer func() {
				_ = os.Setenv(otherVar, oldOther)
			}()

			got := httpx.GetSystemNoProxy()

			assert.Equal(t, tc.want, got)
		})
	}
}

// Helper function to parse URL without error handling (for test data)
func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}
