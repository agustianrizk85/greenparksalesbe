// Command ingest runs the workbook → validate → clean → map → preview pipeline
// against an XLSX file and prints the headline figures, validation summary and
// (optionally) the full mapped dashboard JSON. It is the CLI front-end to the
// internal/ingest engine, used to verify mappings before the HTTP upload flow.
//
// Usage:
//
//	go run ./cmd/ingest "path/to/DASHBOARD SALES_GREENPARK_2026.xlsx" [-json]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"greenpark/sales/internal/ingest"
)

func main() {
	args := os.Args[1:]
	full := false
	var path string
	for _, a := range args {
		if a == "-json" || a == "--json" {
			full = true
			continue
		}
		path = a
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "usage: ingest <workbook.xlsx> [-json]")
		os.Exit(2)
	}

	res, err := ingest.Run(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingest error:", err)
		os.Exit(1)
	}

	h := res.Headline
	fmt.Println("=== PREVIEW: Main Funnel (MASTER DATA_LEADS — semua data, tanpa cleaning) ===")
	fmt.Printf("  Leads            : %d  (semua baris; ditandai: wrong %d, duplikat %d, luar periode %d — tetap dihitung)\n",
		h.Leads, h.DroppedWrong, h.DroppedDup, h.DroppedPeriod)
	fmt.Printf("  Valid Leads      : %d\n", h.ValidLeads)
	fmt.Printf("  Confirmed Visit  : %d\n", h.CV)
	fmt.Printf("  Project Visitor  : %d\n", h.PV)
	fmt.Println("=== PREVIEW: Sales (DATA PENJUALAN) ===")
	fmt.Printf("  Booking (akad+proses): %d\n", h.Booking)
	fmt.Printf("  Akad             : %d\n", h.Akad)
	fmt.Printf("  Proses           : %d\n", h.Proses)
	fmt.Printf("  Batal            : %d\n", h.Batal)
	fmt.Printf("  Cash-In          : Rp %s  (Rp %.2f M)\n", grouped(h.CashIn), float64(h.CashIn)/1e9)
	fmt.Printf("  Visitor walk-ins : %d  (event panel only — di luar funnel)\n", h.VisitorWalkIns)

	fmt.Println("=== ROWS READ ===")
	for _, s := range sortedKeys(res.RowsRead) {
		fmt.Printf("  %-22s: %d\n", s, res.RowsRead[s])
	}

	// Validation summary by check + severity.
	fmt.Printf("=== VALIDATION: %d issue(s) (%d error) ===\n", len(res.Issues), res.ErrorCount())
	byCheck := map[string][3]int{} // [error, warning, info]
	for _, is := range res.Issues {
		c := byCheck[is.Check]
		switch is.Severity {
		case ingest.SevError:
			c[0]++
		case ingest.SevWarning:
			c[1]++
		case ingest.SevInfo:
			c[2]++
		}
		byCheck[is.Check] = c
	}
	for _, k := range sortedKeys3(byCheck) {
		c := byCheck[k]
		fmt.Printf("  %-18s: err=%d warn=%d info=%d\n", k, c[0], c[1], c[2])
	}
	// Show first few concrete issues as examples.
	shown := 0
	for _, is := range res.Issues {
		if is.Severity == ingest.SevInfo {
			continue
		}
		fmt.Printf("    [%s] %s r%d: %s\n", is.Severity, is.Sheet, is.Row, is.Message)
		if shown++; shown >= 8 {
			fmt.Println("    …")
			break
		}
	}

	if full {
		b, _ := json.MarshalIndent(res.Preview, "", "  ")
		fmt.Println("=== FULL PREVIEW (dashboard payload) ===")
		fmt.Println(string(b))
	}
}

func grouped(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func sortedKeys(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sortedKeys3(m map[string][3]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
