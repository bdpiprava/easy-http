package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

// CookieJarManager wraps http.CookieJar with additional utilities for cookie management
type CookieJarManager struct {
	jar        http.CookieJar
	trackedURL sync.Map // map[string]*url.URL - tracks URLs for cookie persistence
}

// NewCookieJarManager creates a new cookie jar manager with public suffix list support
func NewCookieJarManager() (*CookieJarManager, error) {
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cookie jar")
	}
	return &CookieJarManager{
		jar:        jar,
		trackedURL: sync.Map{},
	}, nil
}

// Jar returns the underlying http.CookieJar
func (m *CookieJarManager) Jar() http.CookieJar {
	return m.jar
}

// SetCookies sets cookies for a given URL and tracks it for persistence
func (m *CookieJarManager) SetCookies(u *url.URL, cookies []*http.Cookie) {
	m.jar.SetCookies(u, cookies)
	// Track this URL for persistence
	m.trackedURL.Store(u.String(), u)
}

// GetCookies returns all cookies for a given URL
func (m *CookieJarManager) GetCookies(u *url.URL) []*http.Cookie {
	return m.jar.Cookies(u)
}

// ClearCookies clears all cookies for a given URL by setting an empty list
func (m *CookieJarManager) ClearCookies(u *url.URL) {
	// Get existing cookies and expire them by setting MaxAge to -1
	cookies := m.jar.Cookies(u)
	for _, cookie := range cookies {
		cookie.MaxAge = -1
	}
	m.jar.SetCookies(u, cookies)
}

// ClearAllCookies clears all tracked cookies
func (m *CookieJarManager) ClearAllCookies() {
	m.trackedURL.Range(func(key, value any) bool {
		if u, ok := value.(*url.URL); ok {
			// Get existing cookies and expire them by setting MaxAge to -1
			cookies := m.jar.Cookies(u)
			for _, cookie := range cookies {
				cookie.MaxAge = -1
			}
			m.jar.SetCookies(u, cookies)
		}
		m.trackedURL.Delete(key)
		return true
	})
}

// SaveCookies saves all cookies to a file in JSON format
func (m *CookieJarManager) SaveCookies(filename string) error {
	cookieData := make(map[string][]*SerializableCookie)

	// Iterate through all tracked URLs and get their cookies
	m.trackedURL.Range(func(key, value any) bool {
		urlStr, ok := key.(string)
		if !ok {
			return true
		}

		u, ok := value.(*url.URL)
		if !ok {
			return true
		}

		cookies := m.jar.Cookies(u)
		if len(cookies) == 0 {
			return true
		}

		serializableCookies := make([]*SerializableCookie, len(cookies))
		for i, cookie := range cookies {
			serializableCookies[i] = FromHTTPCookie(cookie)
		}
		cookieData[urlStr] = serializableCookies
		return true
	})

	data, err := json.MarshalIndent(cookieData, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cookies to JSON")
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return errors.Wrapf(err, "failed to write cookies to file: %s", filename)
	}

	return nil
}

// LoadCookies loads cookies from a JSON file
func (m *CookieJarManager) LoadCookies(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to read cookies from file: %s", filename)
	}

	cookieData := make(map[string][]*SerializableCookie)
	if err := json.Unmarshal(data, &cookieData); err != nil {
		return errors.Wrap(err, "failed to unmarshal cookies from JSON")
	}

	for urlStr, serializableCookies := range cookieData {
		u, err := url.Parse(urlStr)
		if err != nil {
			// Skip invalid URLs but continue loading others
			continue
		}

		httpCookies := make([]*http.Cookie, len(serializableCookies))
		for i, sc := range serializableCookies {
			httpCookies[i] = sc.ToHTTPCookie()
		}

		m.SetCookies(u, httpCookies)
	}

	return nil
}

// SerializableCookie is a JSON-serializable version of http.Cookie
type SerializableCookie struct {
	Name       string    `json:"name"`
	Value      string    `json:"value"`
	Path       string    `json:"path,omitempty"`
	Domain     string    `json:"domain,omitempty"`
	Expires    time.Time `json:"expires,omitempty"`
	RawExpires string    `json:"raw_expires,omitempty"`
	MaxAge     int       `json:"max_age,omitempty"`
	Secure     bool      `json:"secure,omitempty"`
	HTTPOnly   bool      `json:"http_only,omitempty"`
	SameSite   string    `json:"same_site,omitempty"`
	Raw        string    `json:"raw,omitempty"`
	Unparsed   []string  `json:"unparsed,omitempty"`
}

// FromHTTPCookie converts http.Cookie to SerializableCookie
func FromHTTPCookie(cookie *http.Cookie) *SerializableCookie {
	sameSite := "Default"
	switch cookie.SameSite {
	case http.SameSiteLaxMode:
		sameSite = "Lax"
	case http.SameSiteStrictMode:
		sameSite = "Strict"
	case http.SameSiteNoneMode:
		sameSite = "None"
	}

	return &SerializableCookie{
		Name:       cookie.Name,
		Value:      cookie.Value,
		Path:       cookie.Path,
		Domain:     cookie.Domain,
		Expires:    cookie.Expires,
		RawExpires: cookie.RawExpires,
		MaxAge:     cookie.MaxAge,
		Secure:     cookie.Secure,
		HTTPOnly:   cookie.HttpOnly,
		SameSite:   sameSite,
		Raw:        cookie.Raw,
		Unparsed:   cookie.Unparsed,
	}
}

// ToHTTPCookie converts SerializableCookie to http.Cookie
func (c *SerializableCookie) ToHTTPCookie() *http.Cookie {
	sameSite := http.SameSiteDefaultMode
	switch c.SameSite {
	case "Lax":
		sameSite = http.SameSiteLaxMode
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	return &http.Cookie{
		Name:       c.Name,
		Value:      c.Value,
		Path:       c.Path,
		Domain:     c.Domain,
		Expires:    c.Expires,
		RawExpires: c.RawExpires,
		MaxAge:     c.MaxAge,
		Secure:     c.Secure,
		HttpOnly:   c.HTTPOnly,
		SameSite:   sameSite,
		Raw:        c.Raw,
		Unparsed:   c.Unparsed,
	}
}
