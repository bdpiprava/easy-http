package httpx

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/proxy"
)

const (
	// Proxy scheme constants
	schemeHTTP   = "http"
	schemeHTTPS  = "https"
	schemeSOCKS4 = "socks4"
	schemeSOCKS5 = "socks5"
)

// ProxyConfig holds proxy configuration settings
type ProxyConfig struct {
	// ProxyURL is the HTTP/HTTPS/SOCKS proxy URL
	ProxyURL *url.URL

	// ProxyFunc allows dynamic proxy selection per request
	ProxyFunc func(*http.Request) (*url.URL, error)

	// NoProxy is a list of domains/IPs to bypass the proxy
	// Supports: exact match, wildcard (*.example.com), CIDR (192.168.0.0/16)
	NoProxy []string

	// ProxyAuth contains credentials for proxy authentication
	ProxyAuth *ProxyAuth
}

// ProxyAuth holds proxy authentication credentials
type ProxyAuth struct {
	Username string
	Password string
}

// GetSystemProxyFunc returns a proxy function that uses system environment variables
// Checks HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables
func GetSystemProxyFunc() func(*http.Request) (*url.URL, error) {
	return http.ProxyFromEnvironment
}

// ParseProxyURL parses and validates a proxy URL
// Supports: http://, https://, socks4://, socks5://
func ParseProxyURL(proxyURL string) (*url.URL, error) {
	if proxyURL == "" {
		return nil, errors.New("proxy URL cannot be empty")
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse proxy URL: %s", proxyURL)
	}

	// Validate proxy scheme
	switch u.Scheme {
	case schemeHTTP, schemeHTTPS, schemeSOCKS4, schemeSOCKS5:
		// Valid schemes
	default:
		return nil, errors.Errorf("unsupported proxy scheme '%s': must be http, https, socks4, or socks5", u.Scheme)
	}

	// Validate host
	if u.Host == "" {
		return nil, errors.Errorf("proxy URL must have a host: %s", proxyURL)
	}

	return u, nil
}

// ShouldBypassProxy checks if a request should bypass the proxy based on NoProxy rules
func ShouldBypassProxy(req *http.Request, noProxyList []string) bool {
	if len(noProxyList) == 0 {
		return false
	}

	host := req.URL.Hostname()
	if host == "" {
		return false
	}

	for _, pattern := range noProxyList {
		if matchesNoProxyPattern(host, pattern) {
			return true
		}
	}

	return false
}

// matchesNoProxyPattern checks if a host matches a no-proxy pattern
// Supports:
// - Exact match: "example.com"
// - Wildcard: "*.example.com"
// - CIDR: "192.168.0.0/16"
// - Special values: "localhost", "127.0.0.1"
func matchesNoProxyPattern(host, pattern string) bool {
	// Trim spaces and convert to lowercase for comparison
	host = strings.ToLower(strings.TrimSpace(host))
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	// Empty pattern matches nothing
	if pattern == "" {
		return false
	}

	// Exact match
	if host == pattern {
		return true
	}

	// Wildcard pattern: *.example.com
	// Matches subdomains but not the parent domain itself
	if strings.HasPrefix(pattern, "*.") {
		domain := pattern[2:] // Remove "*."
		return strings.HasSuffix(host, "."+domain)
	}

	// CIDR pattern: 192.168.0.0/16
	if strings.Contains(pattern, "/") {
		return matchesCIDR(host, pattern)
	}

	// Domain suffix match: .example.com
	if strings.HasPrefix(pattern, ".") {
		return strings.HasSuffix(host, pattern) || host == pattern[1:]
	}

	return false
}

// matchesCIDR checks if a host IP matches a CIDR pattern
func matchesCIDR(host, cidr string) bool {
	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	// Resolve host to IP
	hostIP := net.ParseIP(host)
	if hostIP == nil {
		// Try resolving hostname
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return false
		}
		hostIP = ips[0]
	}

	return ipNet.Contains(hostIP)
}

// CreateProxyFunc creates a proxy function based on ProxyConfig
func CreateProxyFunc(config *ProxyConfig) func(*http.Request) (*url.URL, error) {
	if config == nil {
		return http.ProxyFromEnvironment
	}

	// If custom proxy function is provided, use it
	if config.ProxyFunc != nil {
		return config.ProxyFunc
	}

	// If no proxy URL is configured, use system proxy
	if config.ProxyURL == nil {
		return http.ProxyFromEnvironment
	}

	// Create proxy function with no-proxy support
	return func(req *http.Request) (*url.URL, error) {
		// Check if request should bypass proxy
		if ShouldBypassProxy(req, config.NoProxy) {
			return nil, nil
		}

		return config.ProxyURL, nil
	}
}

// CreateSOCKSDialer creates a SOCKS proxy dialer
func CreateSOCKSDialer(socksURL *url.URL, auth *ProxyAuth) (proxy.Dialer, error) {
	if socksURL == nil {
		return nil, errors.New("SOCKS proxy URL cannot be nil")
	}

	// Extract host and port
	host := socksURL.Host

	// Setup auth if provided
	var socksAuth *proxy.Auth
	if auth != nil && (auth.Username != "" || auth.Password != "") {
		socksAuth = &proxy.Auth{
			User:     auth.Username,
			Password: auth.Password,
		}
	}

	// Create SOCKS dialer based on scheme
	var dialer proxy.Dialer
	var err error

	switch socksURL.Scheme {
	case schemeSOCKS4:
		// SOCKS4 doesn't support authentication
		dialer, err = proxy.SOCKS5("tcp", host, nil, proxy.Direct)
	case schemeSOCKS5:
		dialer, err = proxy.SOCKS5("tcp", host, socksAuth, proxy.Direct)
	default:
		return nil, errors.Errorf("unsupported SOCKS scheme: %s", socksURL.Scheme)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create SOCKS dialer for %s", host)
	}

	return dialer, nil
}

// GetSystemNoProxy returns the NO_PROXY environment variable as a list
func GetSystemNoProxy() []string {
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}

	if noProxy == "" {
		return nil
	}

	// Split by comma and trim spaces
	patterns := strings.Split(noProxy, ",")
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
