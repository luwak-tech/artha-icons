package exchange

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	client *http.Client
	nseURL string
	bseURL string
}

// NewClient returns an Exchange client. If csvURL is empty, uses a default proxy.
func NewClient(nseURL string) *Client {
	if nseURL == "" {
		// A common publicly accessible NSE symbols CSV proxy
		nseURL = "https://archives.nseindia.com/content/equities/EQUITY_L.csv"
	}
	return &Client{
		client: &http.Client{Timeout: 30 * time.Second},
		nseURL: nseURL,
		bseURL: "https://api.bseindia.com/BseIndiaAPI/api/ListofScripData/w?Group=&Scripcode=&industry=&segment=Equity&status=Active",
	}
}

func (c *Client) FetchActiveEquities() ([]Instrument, error) {
	req, err := http.NewRequest("GET", c.nseURL, nil)
	if err != nil {
		return nil, err
	}
	// Add user-agent to bypass basic bot blockers if hitting official routes
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/csv,text/html,application/xhtml+xml,application/xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status fetching CSV: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	// NSE CSV usually has headers, let's read everything
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 1 {
		return []Instrument{}, nil
	}

	var instruments []Instrument
	headers := records[0]

	// Find indexes for SYMBOL, SERIES, ISIN NUMBER, NAME OF COMPANY
	symIdx, serIdx, isinIdx, nameIdx := -1, -1, -1, -1
	for i, h := range headers {
		h = strings.TrimSpace(strings.ToUpper(h))
		if h == "SYMBOL" {
			symIdx = i
		} else if h == "SERIES" {
			serIdx = i
		} else if strings.HasPrefix(h, "ISIN") { // ISIN NUMBER
			isinIdx = i
		} else if strings.HasPrefix(h, "NAME") { // NAME OF COMPANY
			nameIdx = i
		}
	}

	if symIdx == -1 || isinIdx == -1 {
		return nil, fmt.Errorf("could not find SYMBOL and ISIN columns. Found headers: %v", headers)
	}

	// Parse records
	for _, row := range records[1:] {
		if len(row) <= symIdx || len(row) <= isinIdx {
			continue // skip malformed row
		}

		symbol := strings.TrimSpace(row[symIdx])
		isin := strings.TrimSpace(row[isinIdx])

		name := ""
		if nameIdx != -1 && len(row) > nameIdx {
			name = strings.TrimSpace(row[nameIdx])
		}

		// Filter for active equities (typically 'EQ' series in NSE)
		if serIdx != -1 && len(row) > serIdx {
			series := strings.TrimSpace(strings.ToUpper(row[serIdx]))
			if series != "EQ" && series != "" {
				continue
			}
		}

		if symbol != "" && isin != "" {
			instruments = append(instruments, Instrument{
				Symbol:   symbol,
				ISIN:     isin,
				Exchange: "NSE", // Hardcoding NSE for this specific CSV logic
				Name:     name,
				Type:     "equity",
			})
		}
	}

	return instruments, nil
}

func (c *Client) FetchActiveBSEEquities() ([]Instrument, error) {
	req, err := http.NewRequest("GET", c.bseURL, nil)
	if err != nil {
		return nil, err
	}

	// BSE API requires strict headers to prevent 403s
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.bseindia.com/")
	req.Header.Set("Origin", "https://www.bseindia.com")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status fetching BSE JSON: %d", resp.StatusCode)
	}

	var rawData []struct {
		ScripCode  string `json:"SCRIP_CD"`
		ScripID    string `json:"scrip_id"`
		ScripName  string `json:"Scrip_Name"`
		ISIN       string `json:"ISIN_NUMBER"`
		Status     string `json:"Status"`
		Instrument string `json:"Instrument"` // Sometimes "Equity"
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return nil, err
	}

	var instruments []Instrument
	for _, item := range rawData {
		// Filter out suspended or inactive
		if !strings.EqualFold(item.Status, "Active") {
			continue
		}

		isin := strings.TrimSpace(item.ISIN)
		symbol := strings.TrimSpace(item.ScripID)

		if isin != "" && symbol != "" {
			instruments = append(instruments, Instrument{
				Symbol:   symbol,
				ISIN:     isin,
				Exchange: "BSE",
				Name:     strings.TrimSpace(item.ScripName),
				Type:     "equity",
			})
		}
	}

	return instruments, nil
}
