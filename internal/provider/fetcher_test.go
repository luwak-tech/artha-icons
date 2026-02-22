package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetcher_FetchLogo_FaviconFallback(t *testing.T) {
	// 2. Clearbit Autocomplete succeeds
	cbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("query"), "Reliance")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"domain": "ril.com"}]`))
	}))
	defer cbServer.Close()

	// 3. Google Favicon succeeds with a PNG
	favServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("url"), "ril.com")
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		// 8 bytes of fake PNG magic number just to test wrapping
		w.Write([]byte("\x89PNG\x0D\x0A\x1A\x0A"))
	}))
	defer favServer.Close()

	fetcher := NewFetcher()
	fetcher.clearbitAutoURL = cbServer.URL
	fetcher.faviconBaseURL = favServer.URL

	data, err := fetcher.FetchLogo("RELIANCE", "Reliance Industries Limited", "")
	assert.NoError(t, err)

	// It should have wrapped the PNG in an SVG
	assert.Contains(t, string(data), "<svg")
	assert.Contains(t, string(data), "image/png")
}

func TestFetcher_FetchLogo_FaviconFallback_HttpsWww(t *testing.T) {
	// 2. Clearbit Autocomplete succeeds
	cbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"domain": "zensar.com"}]`))
	}))
	defer cbServer.Close()

	// 3. Google Favicon
	//   - Fails on http://zensar.com
	//   - Succeeds on https://www.zensar.com
	favServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "http://zensar.com") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.RawQuery, "https://www.zensar.com") {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("\x89PNG\x0D\x0A\x1A\x0A"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer favServer.Close()

	fetcher := NewFetcher()
	fetcher.clearbitAutoURL = cbServer.URL
	fetcher.faviconBaseURL = favServer.URL

	data, err := fetcher.FetchLogo("ZENSARTECH", "Zensar Technologies Limited", "")
	assert.NoError(t, err)
	assert.Contains(t, string(data), "<svg")
}

func TestFetcher_isSVG(t *testing.T) {
	assert.True(t, isSVG([]byte("<svg></svg>")))
	assert.False(t, isSVG([]byte("PNG data")))
}
