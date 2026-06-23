// Command checksync verifies the Google Sheets live-sync prerequisites:
// that the service-account credential is readable, that the account has access
// to the configured spreadsheet, and that the required source tabs are present.
// It fetches each needed sheet and prints the row count — no dashboard writes.
//
// Usage:
//
//	SALES_GOOGLE_CREDENTIALS=path/to/key.json go run ./cmd/checksync
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"greenpark/sales/internal/config"
	"greenpark/sales/internal/gsheets"
	"greenpark/sales/internal/ingest"
)

// neededSheets mirrors the set the HTTP sync path reads.
var neededSheets = []string{
	"DATA PENJUALAN", "MASTER DATA_LEADS", "MASTER DATA_VISITOR",
	"MASTER STOCK UNIT", "META ADS INPUT",
}

func main() {
	cfg := config.Load()
	fmt.Printf("spreadsheet : %s\n", cfg.GoogleSheetID)
	fmt.Printf("credential  : %s\n", cfg.GoogleCreds)
	if cfg.GoogleCreds == "" {
		fmt.Fprintln(os.Stderr, "ERROR: SALES_GOOGLE_CREDENTIALS belum di-set")
		os.Exit(2)
	}

	gs, err := gsheets.New(cfg.GoogleCreds)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: kredensial tidak terbaca:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	data, err := gs.Fetch(ctx, cfg.GoogleSheetID, neededSheets)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: fetch gagal:", err)
		os.Exit(1)
	}

	if leads, ok := data["MASTER DATA_LEADS"]; ok && len(leads) > 1 {
		for _, j := range []int{10, 11, 12, 13} {
			dist := map[string]int{}
			for _, r := range leads[1:] {
				if j < len(r) {
					if v := strings.TrimSpace(r[j]); v != "" {
						dist[v]++
					}
				}
			}
			fmt.Printf("=== col[%d] non-blank distinct=%d ===\n", j, len(dist))
			for v, c := range dist {
				fmt.Printf("    %5d  %s\n", c, v)
			}
		}
	}

	fmt.Println("=== ROWS FETCHED ===")
	for _, name := range neededSheets {
		rows, ok := data[name]
		if !ok {
			fmt.Printf("  %-22s: (tab tidak ditemukan)\n", name)
			continue
		}
		fmt.Printf("  %-22s: %d baris\n", name, len(rows))
	}

	// Run the full ingest engine over the live sheets — the exact path the
	// HTTP sync uses — so the headline that feeds the snapshot is verified.
	res, err := ingest.RunSheets(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: ingest gagal:", err)
		os.Exit(1)
	}
	h := res.Headline
	fmt.Println("=== HEADLINE (live) ===")
	fmt.Printf("  Booking (akad+proses): %d\n", h.Booking)
	fmt.Printf("  Akad                 : %d\n", h.Akad)
	fmt.Printf("  Proses               : %d\n", h.Proses)
	fmt.Printf("  Batal                : %d\n", h.Batal)
	fmt.Printf("  Cash-In              : Rp %d\n", h.CashIn)
	if res.Preview != nil {
		fmt.Printf("  Ads Spent (exec)     : Rp %d\n", res.Preview.Exec.AdsSpent)
		fmt.Printf("  SaleRows (drill)     : %d\n", len(res.Preview.SaleRows))
		for i, sr := range res.Preview.SaleRows {
			if i >= 3 {
				break
			}
			fmt.Printf("    - proj=%s unit=%q name=%q status=%s\n", sr.Project, sr.Unit, sr.Name, sr.Status)
		}
	}
	fmt.Printf("  Validation issues    : %d (%d error)\n", len(res.Issues), res.ErrorCount())
	fmt.Println("OK")
}
