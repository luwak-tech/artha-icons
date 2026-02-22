package exchange

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_FetchActiveEquities(t *testing.T) {
	// 1. Mock the HTTP server representing a CSV source
	mockCSV := `SYMBOL,NAME OF COMPANY,SERIES,DATE OF LISTING,PAID UP VALUE,MARKET LOT,ISIN NUMBER,FACE VALUE
RELIANCE,Reliance Industries Limited,EQ,29-NOV-1995,10,1,INE001A01036,10
TCS,Tata Consultancy Services Ltd.,EQ,25-AUG-2004,1,1,INE467B01029,1
INVALID,Invalid Series Entry,BE,01-JAN-2000,10,1,INE999Z01099,10
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockCSV))
	}))
	defer server.Close()

	// 2. Initialize the client with the mock URL
	client := NewClient(server.URL)

	// 3. Fetch
	instruments, err := client.FetchActiveEquities()

	// 4. Assertions
	assert.NoError(t, err)
	// Should only parse the 'EQ' series from NSE CSVs typically (or we just extract all based on our logic, let's say all for now or we filter EQ)
	// We will implement logic to filter only 'EQ' (Equity) to avoid bonds/etc if following NSE standards.
	assert.Len(t, instruments, 2)

	rel := instruments[0]
	assert.Equal(t, "RELIANCE", rel.Symbol)
	assert.Equal(t, "INE001A01036", rel.ISIN)
	assert.Equal(t, "NSE", rel.Exchange)

	tcs := instruments[1]
	assert.Equal(t, "TCS", tcs.Symbol)
	assert.Equal(t, "INE467B01029", tcs.ISIN)
}
