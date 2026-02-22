package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	client          *http.Client
	clearbitAutoURL string
	faviconBaseURL  string

	tvFetcher *TVFetcher
}

func NewFetcher() *Fetcher {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	tvFetcher, err := NewTVFetcher("data/tv_urls.txt", client)
	if err != nil {
		// We log but don't strictly fail so the bot can limp along with Clearbit if needed
		fmt.Printf("Warning: Could not initialize TradingView Fetcher: %v\n", err)
	}

	return &Fetcher{
		client:          client,
		clearbitAutoURL: "https://autocomplete.clearbit.com/v1/companies/suggest",
		faviconBaseURL:  "https://t2.gstatic.com/faviconV2",
		tvFetcher:       tvFetcher,
	}
}

// FetchLogo tries multiple sources to find an SVG logo
func (f *Fetcher) FetchLogo(symbol string, name string, exchange string) ([]byte, error) {
	// 0. TradingView WayBack URLs (Primary Dynamic Source)
	if f.tvFetcher != nil {
		if tvUrl, ok := f.tvFetcher.GetLogoURL(symbol, name); ok {
			data, err := f.tvFetcher.Fetch(tvUrl)
			if err == nil && len(data) > 0 {
				// Some TradingView icons are PNGs, we need to wrap them if necessary
				if isSVG(data) {
					return data, nil
				}
				return wrapInSVG(data), nil
			}
		}

		// 0.5 TradingView Live S3 Probing (For unindexed edge cases like SURYALA, SPTRSHI)
		for _, fallbackUrl := range f.tvFetcher.GetLiveFallbackURLs(symbol, name) {
			data, err := f.tvFetcher.Fetch(fallbackUrl)
			if err == nil && len(data) > 0 {
				if isSVG(data) {
					return data, nil
				}
				return wrapInSVG(data), nil
			}
		}

		// 0.6 TradingView Live HTML Scraping (Ultimate Edgecase Finder: e.g., KENVI, SETFNIFBK)
		if logoid, err := f.tvFetcher.ScrapeLiveLogoID(exchange, symbol); err == nil && logoid != "" {
			data, err := f.tvFetcher.Fetch(fmt.Sprintf("https://s3-symbol-logo.tradingview.com/%s--big.svg", logoid))
			if err == nil && len(data) > 0 {
				if isSVG(data) {
					return data, nil
				}
				return wrapInSVG(data), nil
			}

			// Fallback without --big
			data, err = f.tvFetcher.Fetch(fmt.Sprintf("https://s3-symbol-logo.tradingview.com/%s.svg", logoid))
			if err == nil && len(data) > 0 {
				if isSVG(data) {
					return data, nil
				}
				return wrapInSVG(data), nil
			}
		}
	}

	// 2. Clearbit Autocomplete -> Domain -> Favicon -> SVG Wrap
	domain, err := f.getDomain(name)
	if err == nil && domain != "" {
		// First try: http://domain.com
		faviconUrl := fmt.Sprintf("%s?client=SOCIAL&type=FAVICON&fallback_opts=TYPE,SIZE,URL&url=http://%s&size=128", f.faviconBaseURL, domain)
		faviconData, err := f.doFetch(faviconUrl)

		if err != nil || len(faviconData) == 0 {
			// Fallback: https://www.domain.com
			faviconUrl = fmt.Sprintf("%s?client=SOCIAL&type=FAVICON&fallback_opts=TYPE,SIZE,URL&url=https://www.%s&size=128", f.faviconBaseURL, domain)
			faviconData, err = f.doFetch(faviconUrl)
		}

		if err == nil && len(faviconData) > 0 {
			if isSVG(faviconData) {
				return faviconData, nil
			}
			// It's likely PNG or ICO. We wrap it to meet the SVG requirement.
			return wrapInSVG(faviconData), nil
		}
	}

	return nil, fmt.Errorf("logo not found for %s", symbol)
}

func (f *Fetcher) getDomain(name string) (string, error) {
	// First word or two usually gets better matches than the full legal entity name
	query := strings.Split(name, " ")[0]
	url := fmt.Sprintf("%s?query=%s", f.clearbitAutoURL, query)

	data, err := f.doFetch(url)
	if err != nil {
		return "", err
	}

	var suggestions []struct {
		Domain string `json:"domain"`
	}

	if err := json.Unmarshal(data, &suggestions); err != nil {
		return "", err
	}

	if len(suggestions) > 0 {
		return suggestions[0].Domain, nil
	}
	return "", fmt.Errorf("no domain suggestion found")
}

func (f *Fetcher) doFetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "image/svg+xml,image/*,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func isSVG(data []byte) bool {
	return strings.Contains(strings.ToLower(string(data)), "<svg")
}

func wrapInSVG(data []byte) []byte {
	mimeType := http.DetectContentType(data)
	base64Str := base64.StdEncoding.EncodeToString(data)
	svgTmpl := `<svg width="128" height="128" xmlns="http://www.w3.org/2000/svg">
  <image href="data:%s;base64,%s" width="128" height="128"/>
</svg>`
	return []byte(fmt.Sprintf(svgTmpl, mimeType, base64Str))
}
