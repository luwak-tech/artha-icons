# Artha-Icons

Artha-Icons is a high-performance, resilient, and fully automated financial asset logo engine and synchronization service built in Go.

## Use Case & Project Goal
Financial platforms often require standardized, high-quality SVG logos for equities, commodities, and indices. Relying on scraping websites for brand logos is notoriously difficult due to security blocks (like AWS CloudFront 403s on broker CDNs) and missing domains for abstract assets (like CRUDEOIL or NIFTY 50). 

Artha-Icons solves this by:
1. **Dynamic Ingestion:** Parsing the absolute latest, live JSON and CSV endpoints from the NSE (National Stock Exchange) and BSE (Bombay Stock Exchange) to identify all _active_ instruments.
2. **ISIN-First Immutability:** Mapping everything to standard ISINs (e.g. `INE002A01018.svg`) rather than ticker symbols, preventing collisions or broken maps when companies change ticker symbols.
3. **Resilient Provider Fallbacks:**
   - **TradingView (Primary):** Serves as the primary source for retrieving high-quality, pristine SVGs for global and Indian market instruments.
   - **Clearbit API (Fallback):** Falls back to parsing corporate domains and wrapping Favicons into exact `128x128` SVG representations if the primary provider misses an obscure micro-cap.

---

## Stock Coverage

Artha-Icons achieves near-perfect asset coverage for the Indian stock market by combining intelligent matching, deduplication, and cascading fallbacks:

- **Total Active Instruments:** ~6,930 equities tracked across the NSE and BSE.
- **Deduplicated Unique Assets:** ~4,940 unique ISIN-backed companies.
- **Coverage Rate:** Successfully fetches and standardizes **over 99.5%** (~4,915+) of all unique active Indian equities directly into native SVGs.

## Sample Output

When the sync job runs, it automatically populates the configured outputs.

### 1. The Registry (`data/mapping.json`)
A lightweight JSON dictionary that acts as the source of truth, mapping immutable ISINs to their corresponding filenames. This allows your frontend or backend to rapidly serve logos without constantly querying external APIs.
```json
{
  "INE002A01018": "INE002A01018.svg",
  "INE009A01021": "INE009A01021.svg",
  "INE018A01030": "INE018A01030.svg"
}
```

### 2. The Assets (`logos/`)
The physical SVG files are safely downloaded, formatted, and stored locally for your CDN or web-server to host.
```text
logos/
└── equity/
    ├── INE002A01018.svg
    ├── INE009A01021.svg
    └── INE018A01030.svg
```



## Setup & Installation

### Prerequisites
- Go 1.21 or higher installed.
- (Optional) `waybackurls` binary in your PATH. If missing, the app will automatically attempt `go install github.com/tomnomnom/waybackurls@latest` during the first run.

### Running the Project

1. **Clone the Repository:**
   ```bash
   git clone https://github.com/luwak-tech/artha-icons.git
   cd artha-icons
   ```

2. **Download Dependencies:**
   ```bash
   go mod download
   ```

3. **Run the Synchronizer:**
   The sync tool is fully idempotent. You can run it repeatedly, and it will only download "New Listings".
   ```bash
   go run cmd/sync/main.go
   ```

4. **Review Outputs:**
   The logs will display the delta findings and concurrent download progress. Check the `logos/` subdirectories to view the final downloaded SVGs.

---

## Deployment

### Local Docker Compose
To run this continuously or as a background service:
```bash
docker compose up --build -d
```
The volumes are structured to persist the `logos/` and `data/` directories locally without modifying your host system further. 

### Makefile Commands
For rapid local development or execution, a `Makefile` is provided:
- `make build` : Compiles the binary to `bin/artha-icons`
- `make run` : Runs the compiled binary
- `make docker` : Builds the local docker image
- `make run-docker` : Executes a one-off run of the Docker container, automatically mapping your local `logos/` and `data/` directories.

