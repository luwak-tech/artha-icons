package provider

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

type TVFetcher struct {
	urlMap map[string]string
	client *http.Client
}

func NewTVFetcher(cachePath string, client *http.Client) (*TVFetcher, error) {
	tv := &TVFetcher{
		urlMap: make(map[string]string),
		client: client,
	}

	if err := tv.ensureCache(cachePath); err != nil {
		return nil, err
	}

	if err := tv.loadMap(cachePath); err != nil {
		return nil, err
	}

	return tv, nil
}

func (tv *TVFetcher) ensureCache(cachePath string) error {
	if _, err := os.Stat(cachePath); err == nil {
		return nil // Cache exists
	}

	logrus.Infof("TradingView URL cache not found at %s. Generating via waybackurls (this may take a minute)...", cachePath)

	// Create required directories
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}

	// Make sure waybackurls is installed
	if _, err := exec.LookPath("waybackurls"); err != nil {
		logrus.Info("waybackurls not found in PATH. Attempting to install via go install...")
		installCmd := exec.Command("go", "install", "github.com/tomnomnom/waybackurls@latest")
		installCmd.Env = append(os.Environ(), "GOPATH="+filepath.Join(os.Getenv("HOME"), "go"))
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install waybackurls: %v", err)
		}
	}

	goPathBin := filepath.Join(os.Getenv("HOME"), "go", "bin")
	waybackPath := filepath.Join(goPathBin, "waybackurls")

	if _, err := os.Stat(waybackPath); os.IsNotExist(err) {
		waybackPath = "waybackurls" // Try global if not in go/bin
	}

	cmd := exec.Command(waybackPath, "s3-symbol-logo.tradingview.com")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+goPathBin)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run waybackurls: %v", err)
	}

	if err := os.WriteFile(cachePath, out.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to save waybackurls output: %v", err)
	}

	logrus.Infof("Waybackurls cache generated at %s", cachePath)
	return nil
}

func (tv *TVFetcher) loadMap(cachePath string) error {
	file, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url == "" || !strings.HasPrefix(url, "https://s3-symbol-logo.tradingview.com/") {
			continue
		}

		// We want to keep track of if it's an index or just a root file
		pathPart := strings.TrimPrefix(url, "https://s3-symbol-logo.tradingview.com/")
		pathPart = strings.Split(pathPart, "?")[0] // strip params

		if strings.HasSuffix(pathPart, ".svg") || strings.HasSuffix(pathPart, ".png") {
			tv.urlMap[strings.ToLower(pathPart)] = url
		}
	}

	logrus.Infof("Loaded %d TradingView asset URLs into memory map", len(tv.urlMap))
	return scanner.Err()
}

func (tv *TVFetcher) getSlugs(name string) []string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, ",", "")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "&", "and")

	rawSlug := strings.ReplaceAll(name, " ", "-")

	noLtd := strings.ReplaceAll(name, " limited", "")
	noLtd = strings.ReplaceAll(noLtd, " ltd", "")
	noLtdSlug := strings.ReplaceAll(noLtd, " ", "-")

	firstWord := strings.Split(name, " ")[0]
	firstWordSlug := strings.ReplaceAll(firstWord, " ", "-")

	parts := strings.Split(name, " ")
	secondWordSlug := ""
	if len(parts) > 1 {
		secondWordSlug = parts[0] + "-" + parts[1]
	}

	return []string{rawSlug, noLtdSlug, firstWordSlug, secondWordSlug}
}

func (tv *TVFetcher) GetLogoURL(symbol string, name string) (string, bool) {
	symbolLower := strings.ToLower(symbol)
	slugs := append([]string{symbolLower}, tv.getSlugs(name)...)

	var candidates []string
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		// List of possible filenames TradingView might use
		candidates = append(candidates,
			fmt.Sprintf("%s.svg", slug),
			fmt.Sprintf("%s--big.svg", slug),
			fmt.Sprintf("%s--600.png", slug),
			// Check indices subdirectory
			fmt.Sprintf("indices/%s.svg", slug),
			fmt.Sprintf("indices/%s--big.svg", slug),
			fmt.Sprintf("indices/%s--600.png", slug),
		)
	}

	for _, cand := range candidates {
		if url, exists := tv.urlMap[cand]; exists {
			return url, true
		}
	}

	// Fuzzy Fallback Matcher:
	// If it didn't perfectly match any candidate permutations, we try to find a file
	// that starts with the company's first word or the symbol followed by a hyphen.
	// E.g., symbol "RELIANCE" -> matches "reliance-industries--big.svg"
	// E.g., first word "suryalata" -> matches "suryalata-spinning-mills--big.svg"

	firstWord := strings.Split(strings.ToLower(name), " ")[0]
	firstWord = strings.ReplaceAll(firstWord, ".", "")
	firstWord = strings.ReplaceAll(firstWord, ",", "")

	parts := strings.Split(strings.ToLower(name), " ")
	if len(parts) >= 2 {
		secondWord := strings.ReplaceAll(parts[1], ".", "")
		secondWord = strings.ReplaceAll(secondWord, ",", "")
		twoWordPrefix := firstWord + "-" + secondWord + "-"
		for key, url := range tv.urlMap {
			if strings.HasPrefix(key, twoWordPrefix) {
				return url, true
			}
		}
	}

	if len(firstWord) >= 4 {
		prefix := firstWord + "-"
		for key, url := range tv.urlMap {
			if strings.HasPrefix(key, prefix) {
				return url, true
			}
		}
	}

	if len(symbolLower) >= 4 {
		prefix := symbolLower + "-"
		for key, url := range tv.urlMap {
			if strings.HasPrefix(key, prefix) {
				return url, true
			}
		}
	}

	return "", false
}

func (tv *TVFetcher) GetLiveFallbackURLs(symbol string, name string) []string {
	symbolLower := strings.ToLower(symbol)
	slugs := append([]string{symbolLower}, tv.getSlugs(name)...)

	var urls []string
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		urls = append(urls,
			fmt.Sprintf("https://s3-symbol-logo.tradingview.com/%s--big.svg", slug),
			fmt.Sprintf("https://s3-symbol-logo.tradingview.com/%s.svg", slug),
		)
	}
	return urls
}

func (tv *TVFetcher) ScrapeLiveLogoID(exchange string, symbol string) (string, error) {
	// e.g. https://in.tradingview.com/symbols/BSE-KENVI/
	url := fmt.Sprintf("https://in.tradingview.com/symbols/%s-%s/", strings.ToUpper(exchange), strings.ToUpper(symbol))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	// TradingView protects against bare requests, User-Agent is required.
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html")

	resp, err := tv.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Looks for: "logoid":"kenvi-jewels-limited"
	matches := regexp.MustCompile(`"logoid":"([^"]+)"`).FindStringSubmatch(string(body))
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}

	return "", fmt.Errorf("logoid not found in HTML")
}

func (tv *TVFetcher) Fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// MAGIC BYPASS HEADERS FOR TRADINGVIEW S3
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Origin", "https://www.tradingview.com")
	req.Header.Set("Referer", "https://www.tradingview.com/")

	resp, err := tv.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status from TradingView S3: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
