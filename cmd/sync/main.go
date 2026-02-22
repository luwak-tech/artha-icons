package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/artha-icons/internal/exchange"
	"github.com/artha-icons/internal/provider"
	"github.com/artha-icons/internal/storage"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logrus.Info("Starting Artha-Icons Sync Process...")
	startTime := time.Now()

	store, err := storage.New("data/mapping.json")
	if err != nil {
		logrus.Fatalf("Failed to initialize storage: %v", err)
	}

	client := exchange.NewClient("")
	fetcher := provider.NewFetcher()

	logrus.Info("Fetching active instruments from NSE...")
	instruments, err := client.FetchActiveEquities()
	if err != nil {
		logrus.Fatalf("Failed to fetch instruments: %v", err)
	}

	logrus.Infof("Found %d active NSE equities.", len(instruments))

	logrus.Info("Fetching active instruments from BSE...")
	bseInstruments, err := client.FetchActiveBSEEquities()
	if err != nil {
		logrus.Warnf("Failed to fetch BSE instruments (continuing with NSE only): %v", err)
	} else {
		logrus.Infof("Found %d active BSE equities.", len(bseInstruments))
		instruments = append(instruments, bseInstruments...)
	}

	// Pre-filter deduplication to handle dual-listings (e.g. Reliance on NSE and BSE)
	seenISINs := make(map[string]bool)
	var uniqueInstruments []exchange.Instrument
	var duplicatesCount int

	for _, inst := range instruments {
		if _, exists := seenISINs[inst.ISIN]; !exists {
			seenISINs[inst.ISIN] = true
			uniqueInstruments = append(uniqueInstruments, inst)
		} else {
			duplicatesCount++
		}
	}

	var newListings []exchange.Instrument
	for _, inst := range uniqueInstruments {
		if !store.Has(inst.ISIN) {
			newListings = append(newListings, inst)
		}
	}

	logrus.Infof("Delta Analysis: %d Unique Assets, %d Duplicates Skipped.", len(uniqueInstruments), duplicatesCount)
	logrus.Infof("Delta Analysis: %d New Listings pending download.", len(newListings))

	if len(newListings) == 0 {
		logrus.Info("No new listings to sync. Exiting.")
		return
	}

	var wg sync.WaitGroup
	// Semaphore to limit concurrency preventing strict rate limiting
	sem := make(chan struct{}, 10)

	var successCount int
	var mu sync.Mutex

	for _, inst := range newListings {
		wg.Add(1)
		go func(instrument exchange.Instrument) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			logrus.Infof("Fetching logo for [%s] (%s)...", instrument.ISIN, instrument.Symbol)

			data, err := fetcher.FetchLogo(instrument.Symbol, instrument.Name, instrument.Exchange)
			if err != nil {
				logrus.Warnf("Failed logo for [%s] (%s): %v", instrument.ISIN, instrument.Symbol, err)
				return
			}

			filename := fmt.Sprintf("%s.svg", instrument.ISIN)
			folder := instrument.Type
			if folder == "" {
				folder = "equity"
			}
			path := filepath.Join("logos", folder, filename)

			// Optional: sanitize SVG data here

			// Ensure directory exists
			os.MkdirAll(filepath.Dir(path), 0755)

			if err := os.WriteFile(path, data, 0644); err != nil {
				logrus.Errorf("Failed to save logo for [%s]: %v", instrument.ISIN, err)
				return
			}

			store.Add(instrument.ISIN, filename)

			mu.Lock()
			successCount++
			mu.Unlock()

			logrus.Infof("Successfully saved logo for [%s] (%s)", instrument.ISIN, instrument.Symbol)

		}(inst)
	}

	wg.Wait()

	if err := store.Save(); err != nil {
		logrus.Errorf("Failed to save mapping: %v", err)
	}

	logrus.Info("---------------------------------------------------")
	logrus.Info("SYNC REPORT:")
	logrus.Infof("- Total Raw Instruments (NSE+BSE): %d", len(instruments))
	logrus.Infof("- Unique Assets (Deduplicated): %d", len(uniqueInstruments))
	logrus.Infof("- Dual-Listings/Duplicates Skipped: %d", duplicatesCount)
	logrus.Infof("- Grand Total Assets Synced & Stored: %d", len(store.Mapping))
	logrus.Infof("- New Downloads in this run: %d", successCount)
	logrus.Infof("- Time Taken: %s", time.Since(startTime))
	logrus.Info("---------------------------------------------------")
}
