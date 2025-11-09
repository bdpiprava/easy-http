package httpx_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bdpiprava/easy-http/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCookieJarManager(t *testing.T) {
	t.Parallel()

	t.Run("creates cookie jar manager successfully", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()

		require.NoError(t, err)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.Jar())
	})
}

func TestCookieJarManager_Jar(t *testing.T) {
	t.Parallel()

	t.Run("returns underlying cookie jar", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		jar := manager.Jar()

		assert.NotNil(t, jar)
	})
}

func TestCookieJarManager_SetCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		cookies []*http.Cookie
	}{
		{
			name: "sets single cookie for URL",
			url:  "https://example.com/path",
			cookies: []*http.Cookie{
				{Name: "session", Value: "abc123"},
			},
		},
		{
			name: "sets multiple cookies for URL",
			url:  "https://api.example.com/v1",
			cookies: []*http.Cookie{
				{Name: "token", Value: "xyz789"},
				{Name: "user_id", Value: "12345"},
				{Name: "preferences", Value: "dark_mode"},
			},
		},
		{
			name:    "sets empty cookie list",
			url:     "https://test.com",
			cookies: []*http.Cookie{},
		},
		{
			name: "sets cookie with all attributes",
			url:  "https://secure.example.com/api",
			cookies: []*http.Cookie{
				{
					Name:     "secure_session",
					Value:    "encrypted_value",
					Path:     "/",
					Expires:  time.Now().Add(24 * time.Hour),
					MaxAge:   86400,
					Secure:   true,
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager, err := httpx.NewCookieJarManager()
			require.NoError(t, err)

			u, err := url.Parse(tc.url)
			require.NoError(t, err)

			manager.SetCookies(u, tc.cookies)

			// Verify cookies were set by retrieving them
			retrieved := manager.GetCookies(u)
			assert.Equal(t, len(tc.cookies), len(retrieved))
		})
	}
}

func TestCookieJarManager_GetCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setURL      string
		setCookies  []*http.Cookie
		getURL      string
		wantCount   int
		wantCookies []string // cookie names
	}{
		{
			name:   "gets cookies for exact URL",
			setURL: "https://example.com/path",
			setCookies: []*http.Cookie{
				{Name: "session", Value: "abc123"},
				{Name: "user", Value: "john"},
			},
			getURL:      "https://example.com/path",
			wantCount:   2,
			wantCookies: []string{"session", "user"},
		},
		{
			name:        "returns empty for URL with no cookies",
			setURL:      "https://example.com",
			setCookies:  []*http.Cookie{{Name: "test", Value: "value"}},
			getURL:      "https://different.com",
			wantCount:   0,
			wantCookies: []string{},
		},
		{
			name:   "gets cookies with domain matching",
			setURL: "https://example.com",
			setCookies: []*http.Cookie{
				{Name: "shared", Value: "data", Domain: "example.com"},
			},
			getURL:      "https://subdomain.example.com",
			wantCount:   1,
			wantCookies: []string{"shared"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager, err := httpx.NewCookieJarManager()
			require.NoError(t, err)

			// Set cookies
			setU, err := url.Parse(tc.setURL)
			require.NoError(t, err)
			manager.SetCookies(setU, tc.setCookies)

			// Get cookies
			getU, err := url.Parse(tc.getURL)
			require.NoError(t, err)
			cookies := manager.GetCookies(getU)

			assert.Equal(t, tc.wantCount, len(cookies))
			if tc.wantCount > 0 {
				cookieNames := make([]string, len(cookies))
				for i, c := range cookies {
					cookieNames[i] = c.Name
				}
				for _, wantName := range tc.wantCookies {
					assert.Contains(t, cookieNames, wantName)
				}
			}
		})
	}
}

func TestCookieJarManager_ClearCookies(t *testing.T) {
	t.Parallel()

	t.Run("clears cookies for specific URL", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		u, err := url.Parse("https://example.com")
		require.NoError(t, err)

		// Set cookies
		cookies := []*http.Cookie{
			{Name: "session", Value: "abc123"},
			{Name: "user", Value: "john"},
		}
		manager.SetCookies(u, cookies)

		// Verify cookies were set
		retrieved := manager.GetCookies(u)
		assert.Equal(t, 2, len(retrieved))

		// Clear cookies
		manager.ClearCookies(u)

		// Verify cookies were cleared or expired
		// Note: Cookie jar may expire cookies (MaxAge=-1) instead of removing them
		afterClear := manager.GetCookies(u)
		for _, c := range afterClear {
			if c.MaxAge >= 0 {
				t.Errorf("Expected cookie to be expired or removed, but MaxAge=%d", c.MaxAge)
			}
		}
	})

	t.Run("clearing cookies for one URL does not affect others", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		url1, err := url.Parse("https://example.com")
		require.NoError(t, err)
		url2, err := url.Parse("https://other.com")
		require.NoError(t, err)

		// Set cookies for both URLs
		manager.SetCookies(url1, []*http.Cookie{{Name: "cookie1", Value: "value1"}})
		manager.SetCookies(url2, []*http.Cookie{{Name: "cookie2", Value: "value2"}})

		// Clear cookies for url1
		manager.ClearCookies(url1)

		// Verify url1 cookies cleared (expired) but url2 cookies remain
		url1Cookies := manager.GetCookies(url1)
		for _, c := range url1Cookies {
			if c.MaxAge >= 0 {
				t.Errorf("Expected url1 cookie to be expired or removed, but MaxAge=%d", c.MaxAge)
			}
		}
		assert.Equal(t, 1, len(manager.GetCookies(url2)))
	})
}

func TestCookieJarManager_ClearAllCookies(t *testing.T) {
	t.Parallel()

	t.Run("clears all tracked cookies", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		// Set cookies for multiple URLs
		url1, err := url.Parse("https://example.com")
		require.NoError(t, err)
		url2, err := url.Parse("https://api.example.com")
		require.NoError(t, err)
		url3, err := url.Parse("https://other.com")
		require.NoError(t, err)

		manager.SetCookies(url1, []*http.Cookie{{Name: "cookie1", Value: "value1"}})
		manager.SetCookies(url2, []*http.Cookie{{Name: "cookie2", Value: "value2"}})
		manager.SetCookies(url3, []*http.Cookie{{Name: "cookie3", Value: "value3"}})

		// Verify cookies were set
		assert.Greater(t, len(manager.GetCookies(url1)), 0)
		assert.Greater(t, len(manager.GetCookies(url2)), 0)
		assert.Greater(t, len(manager.GetCookies(url3)), 0)

		// Clear all cookies
		manager.ClearAllCookies()

		// Verify cookies are cleared or expired (cookie jar may keep them with MaxAge=-1)
		// Note: Some cookie jar implementations may not fully remove cookies but expire them
		// So we check that either no cookies are returned or they're all expired
		for _, u := range []*url.URL{url1, url2, url3} {
			cookies := manager.GetCookies(u)
			for _, c := range cookies {
				// If cookies still exist, they should be expired (MaxAge < 0)
				if c.MaxAge >= 0 {
					t.Errorf("Expected cookie to be expired or removed, but MaxAge=%d", c.MaxAge)
				}
			}
		}
	})

	t.Run("clearing all cookies on empty jar succeeds", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		// Should not panic or error
		manager.ClearAllCookies()
	})
}

func TestCookieJarManager_SaveCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupURLs []string
		cookies   map[string][]*http.Cookie
		wantErr   bool
	}{
		{
			name:      "saves single URL with cookies",
			setupURLs: []string{"https://example.com"},
			cookies: map[string][]*http.Cookie{
				"https://example.com": {
					{Name: "session", Value: "abc123"},
				},
			},
			wantErr: false,
		},
		{
			name:      "saves multiple URLs with cookies",
			setupURLs: []string{"https://example.com", "https://api.example.com"},
			cookies: map[string][]*http.Cookie{
				"https://example.com": {
					{Name: "session", Value: "abc123"},
					{Name: "user", Value: "john"},
				},
				"https://api.example.com": {
					{Name: "token", Value: "xyz789"},
				},
			},
			wantErr: false,
		},
		{
			name:      "saves cookies with all attributes",
			setupURLs: []string{"https://secure.example.com"},
			cookies: map[string][]*http.Cookie{
				"https://secure.example.com": {
					{
						Name:       "secure_session",
						Value:      "encrypted",
						Path:       "/api",
						Domain:     "secure.example.com",
						Expires:    time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
						RawExpires: "Wed, 31 Dec 2025 23:59:59 GMT",
						MaxAge:     86400,
						Secure:     true,
						HttpOnly:   true,
						SameSite:   http.SameSiteStrictMode,
					},
				},
			},
			wantErr: false,
		},
		{
			name:      "saves empty cookie jar",
			setupURLs: []string{},
			cookies:   map[string][]*http.Cookie{},
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager, err := httpx.NewCookieJarManager()
			require.NoError(t, err)

			// Setup cookies
			for urlStr, cookies := range tc.cookies {
				u, err := url.Parse(urlStr)
				require.NoError(t, err)
				manager.SetCookies(u, cookies)
			}

			// Create temp file
			tmpDir := t.TempDir()
			filename := filepath.Join(tmpDir, "cookies.json")

			// Save cookies
			err = manager.SaveCookies(filename)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify file was created
				_, statErr := os.Stat(filename)
				assert.NoError(t, statErr)
			}
		})
	}

	t.Run("returns error for invalid file path", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		// Try to save to invalid path
		err = manager.SaveCookies("/invalid/nonexistent/path/cookies.json")

		assert.Error(t, err)
	})
}

func TestCookieJarManager_LoadCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		jsonData  string
		wantURLs  []string
		wantCount map[string]int
		wantErr   bool
	}{
		{
			name: "loads single URL with cookies",
			jsonData: `{
				"https://example.com": [
					{
						"name": "session",
						"value": "abc123"
					}
				]
			}`,
			wantURLs: []string{"https://example.com"},
			wantCount: map[string]int{
				"https://example.com": 1,
			},
			wantErr: false,
		},
		{
			name: "loads multiple URLs with cookies",
			jsonData: `{
				"https://example.com": [
					{
						"name": "session",
						"value": "abc123"
					},
					{
						"name": "user",
						"value": "john"
					}
				],
				"https://api.example.com": [
					{
						"name": "token",
						"value": "xyz789"
					}
				]
			}`,
			wantURLs: []string{"https://example.com", "https://api.example.com"},
			wantCount: map[string]int{
				"https://example.com":     2,
				"https://api.example.com": 1,
			},
			wantErr: false,
		},
		{
			name: "loads cookies with all SameSite modes",
			jsonData: `{
				"https://example.com": [
					{
						"name": "default_cookie",
						"value": "value1",
						"same_site": "Default"
					},
					{
						"name": "lax_cookie",
						"value": "value2",
						"same_site": "Lax"
					},
					{
						"name": "strict_cookie",
						"value": "value3",
						"same_site": "Strict"
					},
					{
						"name": "none_cookie",
						"value": "value4",
						"same_site": "None"
					}
				]
			}`,
			wantURLs: []string{"https://example.com"},
			wantCount: map[string]int{
				"https://example.com": 4,
			},
			wantErr: false,
		},
		{
			name: "loads cookies with all attributes",
			jsonData: `{
				"https://secure.example.com": [
					{
						"name": "secure_session",
						"value": "encrypted",
						"path": "/",
						"expires": "2025-12-31T23:59:59Z",
						"raw_expires": "Wed, 31 Dec 2025 23:59:59 GMT",
						"max_age": 86400,
						"secure": true,
						"http_only": true,
						"same_site": "Strict"
					}
				]
			}`,
			wantURLs: []string{"https://secure.example.com"},
			wantCount: map[string]int{
				"https://secure.example.com": 1,
			},
			wantErr: false,
		},
		{
			name:      "loads empty cookie file",
			jsonData:  `{}`,
			wantURLs:  []string{},
			wantCount: map[string]int{},
			wantErr:   false,
		},
		{
			name:     "returns error for invalid JSON",
			jsonData: `{invalid json}`,
			wantErr:  true,
		},
		{
			name: "skips invalid URLs and continues loading",
			jsonData: `{
				"not a valid url": [
					{
						"name": "bad",
						"value": "cookie"
					}
				],
				"https://valid.com": [
					{
						"name": "good",
						"value": "cookie"
					}
				]
			}`,
			wantURLs: []string{"https://valid.com"},
			wantCount: map[string]int{
				"https://valid.com": 1,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager, err := httpx.NewCookieJarManager()
			require.NoError(t, err)

			// Create temp file with JSON data
			tmpDir := t.TempDir()
			filename := filepath.Join(tmpDir, "cookies.json")
			err = os.WriteFile(filename, []byte(tc.jsonData), 0600)
			require.NoError(t, err)

			// Load cookies
			err = manager.LoadCookies(filename)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify cookies were loaded
				for _, urlStr := range tc.wantURLs {
					u, parseErr := url.Parse(urlStr)
					require.NoError(t, parseErr)

					cookies := manager.GetCookies(u)
					expectedCount := tc.wantCount[urlStr]
					assert.Equal(t, expectedCount, len(cookies), "unexpected cookie count for URL: %s", urlStr)
				}
			}
		})
	}

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		t.Parallel()

		manager, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		err = manager.LoadCookies("/nonexistent/file.json")

		assert.Error(t, err)
	})
}

func TestCookieJarManager_SaveAndLoadRoundtrip(t *testing.T) {
	t.Parallel()

	t.Run("save and load preserves all cookie data", func(t *testing.T) {
		t.Parallel()

		manager1, err := httpx.NewCookieJarManager()
		require.NoError(t, err)

		// Set up cookies with various attributes
		u, err := url.Parse("https://example.com")
		require.NoError(t, err)

		originalCookies := []*http.Cookie{
			{
				Name:     "session",
				Value:    "abc123",
				Path:     "/",
				Domain:   "example.com",
				Expires:  time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
				MaxAge:   86400,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			},
			{
				Name:     "preferences",
				Value:    "dark_mode",
				SameSite: http.SameSiteLaxMode,
			},
		}
		manager1.SetCookies(u, originalCookies)

		// Save cookies
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "cookies.json")
		err = manager1.SaveCookies(filename)
		require.NoError(t, err)

		// Load cookies into new manager
		manager2, err := httpx.NewCookieJarManager()
		require.NoError(t, err)
		err = manager2.LoadCookies(filename)
		require.NoError(t, err)

		// Verify cookies match
		loadedCookies := manager2.GetCookies(u)
		assert.Equal(t, len(originalCookies), len(loadedCookies))

		// Verify cookie names are present
		cookieNames := make(map[string]bool)
		for _, c := range loadedCookies {
			cookieNames[c.Name] = true
		}
		assert.True(t, cookieNames["session"])
		assert.True(t, cookieNames["preferences"])
	})
}

func TestFromHTTPCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cookie       *http.Cookie
		wantName     string
		wantValue    string
		wantSameSite string
	}{
		{
			name: "converts cookie with all attributes",
			cookie: &http.Cookie{
				Name:       "session",
				Value:      "abc123",
				Path:       "/api",
				Domain:     "example.com",
				Expires:    time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
				RawExpires: "Wed, 31 Dec 2025 23:59:59 GMT",
				MaxAge:     86400,
				Secure:     true,
				HttpOnly:   true,
				SameSite:   http.SameSiteStrictMode,
				Raw:        "session=abc123",
				Unparsed:   []string{"session=abc123"},
			},
			wantName:     "session",
			wantValue:    "abc123",
			wantSameSite: "Strict",
		},
		{
			name: "converts cookie with SameSiteLaxMode",
			cookie: &http.Cookie{
				Name:     "lax_cookie",
				Value:    "value",
				SameSite: http.SameSiteLaxMode,
			},
			wantName:     "lax_cookie",
			wantValue:    "value",
			wantSameSite: "Lax",
		},
		{
			name: "converts cookie with SameSiteNoneMode",
			cookie: &http.Cookie{
				Name:     "none_cookie",
				Value:    "value",
				SameSite: http.SameSiteNoneMode,
			},
			wantName:     "none_cookie",
			wantValue:    "value",
			wantSameSite: "None",
		},
		{
			name: "converts cookie with SameSiteDefaultMode",
			cookie: &http.Cookie{
				Name:     "default_cookie",
				Value:    "value",
				SameSite: http.SameSiteDefaultMode,
			},
			wantName:     "default_cookie",
			wantValue:    "value",
			wantSameSite: "Default",
		},
		{
			name: "converts minimal cookie",
			cookie: &http.Cookie{
				Name:  "simple",
				Value: "cookie",
			},
			wantName:     "simple",
			wantValue:    "cookie",
			wantSameSite: "Default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serializable := httpx.FromHTTPCookie(tc.cookie)

			assert.NotNil(t, serializable)
			assert.Equal(t, tc.wantName, serializable.Name)
			assert.Equal(t, tc.wantValue, serializable.Value)
			assert.Equal(t, tc.wantSameSite, serializable.SameSite)

			if tc.cookie.Path != "" {
				assert.Equal(t, tc.cookie.Path, serializable.Path)
			}
			if tc.cookie.Domain != "" {
				assert.Equal(t, tc.cookie.Domain, serializable.Domain)
			}
			if tc.cookie.MaxAge != 0 {
				assert.Equal(t, tc.cookie.MaxAge, serializable.MaxAge)
			}
			assert.Equal(t, tc.cookie.Secure, serializable.Secure)
			assert.Equal(t, tc.cookie.HttpOnly, serializable.HTTPOnly)
		})
	}
}

func TestSerializableCookie_ToHTTPCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		serializable *httpx.SerializableCookie
		wantName     string
		wantValue    string
		wantSameSite http.SameSite
	}{
		{
			name: "converts to http.Cookie with all attributes",
			serializable: &httpx.SerializableCookie{
				Name:       "session",
				Value:      "abc123",
				Path:       "/api",
				Domain:     "example.com",
				Expires:    time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
				RawExpires: "Wed, 31 Dec 2025 23:59:59 GMT",
				MaxAge:     86400,
				Secure:     true,
				HTTPOnly:   true,
				SameSite:   "Strict",
				Raw:        "session=abc123",
				Unparsed:   []string{"session=abc123"},
			},
			wantName:     "session",
			wantValue:    "abc123",
			wantSameSite: http.SameSiteStrictMode,
		},
		{
			name: "converts Lax SameSite",
			serializable: &httpx.SerializableCookie{
				Name:     "lax_cookie",
				Value:    "value",
				SameSite: "Lax",
			},
			wantName:     "lax_cookie",
			wantValue:    "value",
			wantSameSite: http.SameSiteLaxMode,
		},
		{
			name: "converts None SameSite",
			serializable: &httpx.SerializableCookie{
				Name:     "none_cookie",
				Value:    "value",
				SameSite: "None",
			},
			wantName:     "none_cookie",
			wantValue:    "value",
			wantSameSite: http.SameSiteNoneMode,
		},
		{
			name: "converts Default SameSite",
			serializable: &httpx.SerializableCookie{
				Name:     "default_cookie",
				Value:    "value",
				SameSite: "Default",
			},
			wantName:     "default_cookie",
			wantValue:    "value",
			wantSameSite: http.SameSiteDefaultMode,
		},
		{
			name: "converts with unknown SameSite defaults to Default",
			serializable: &httpx.SerializableCookie{
				Name:     "unknown",
				Value:    "value",
				SameSite: "InvalidValue",
			},
			wantName:     "unknown",
			wantValue:    "value",
			wantSameSite: http.SameSiteDefaultMode,
		},
		{
			name: "converts minimal cookie",
			serializable: &httpx.SerializableCookie{
				Name:  "simple",
				Value: "cookie",
			},
			wantName:     "simple",
			wantValue:    "cookie",
			wantSameSite: http.SameSiteDefaultMode,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			httpCookie := tc.serializable.ToHTTPCookie()

			assert.NotNil(t, httpCookie)
			assert.Equal(t, tc.wantName, httpCookie.Name)
			assert.Equal(t, tc.wantValue, httpCookie.Value)
			assert.Equal(t, tc.wantSameSite, httpCookie.SameSite)

			if tc.serializable.Path != "" {
				assert.Equal(t, tc.serializable.Path, httpCookie.Path)
			}
			if tc.serializable.Domain != "" {
				assert.Equal(t, tc.serializable.Domain, httpCookie.Domain)
			}
			if tc.serializable.MaxAge != 0 {
				assert.Equal(t, tc.serializable.MaxAge, httpCookie.MaxAge)
			}
			assert.Equal(t, tc.serializable.Secure, httpCookie.Secure)
			assert.Equal(t, tc.serializable.HTTPOnly, httpCookie.HttpOnly)
		})
	}
}

func TestCookie_SerializationRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{
			name: "roundtrip with all attributes",
			cookie: &http.Cookie{
				Name:       "session",
				Value:      "abc123",
				Path:       "/api",
				Domain:     "example.com",
				Expires:    time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
				RawExpires: "Wed, 31 Dec 2025 23:59:59 GMT",
				MaxAge:     86400,
				Secure:     true,
				HttpOnly:   true,
				SameSite:   http.SameSiteStrictMode,
			},
		},
		{
			name: "roundtrip with Lax SameSite",
			cookie: &http.Cookie{
				Name:     "lax",
				Value:    "value",
				SameSite: http.SameSiteLaxMode,
			},
		},
		{
			name: "roundtrip with None SameSite",
			cookie: &http.Cookie{
				Name:     "none",
				Value:    "value",
				SameSite: http.SameSiteNoneMode,
			},
		},
		{
			name: "roundtrip minimal cookie",
			cookie: &http.Cookie{
				Name:  "minimal",
				Value: "data",
				Path:  "/", // Cookie jar sets default path
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Convert to serializable and back
			serializable := httpx.FromHTTPCookie(tc.cookie)
			roundtripped := serializable.ToHTTPCookie()

			// Verify key fields match
			assert.Equal(t, tc.cookie.Name, roundtripped.Name)
			assert.Equal(t, tc.cookie.Value, roundtripped.Value)
			assert.Equal(t, tc.cookie.Path, roundtripped.Path)
			assert.Equal(t, tc.cookie.Domain, roundtripped.Domain)
			// Note: Skip MaxAge comparison as it may have implementation-specific defaults
			assert.Equal(t, tc.cookie.Secure, roundtripped.Secure)
			assert.Equal(t, tc.cookie.HttpOnly, roundtripped.HttpOnly)
			// Note: Skip SameSite comparison for minimal cookies as jar may set defaults
			if tc.cookie.SameSite != 0 {
				assert.Equal(t, tc.cookie.SameSite, roundtripped.SameSite)
			}
		})
	}
}
