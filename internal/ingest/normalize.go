// Package ingest turns the "DASHBOARD SALES_GREENPARK" workbook into a validated,
// cleaned, mapped dashboard preview. The flow mirrors the product spec:
//
//	Upload XLSX → Validasi → Cleaning → Mapping → Preview → (Approve) → Dashboard
//
// This file holds the field-level normalizers used by every sheet mapper:
// phone numbers, Indonesian/serial dates, Rupiah amounts and project codes.
package ingest

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CanonicalProjects is the master project list (KODE PROJECT column of the
// MASTER STOCK UNIT sheet). Any project code outside this set is flagged by the
// "Nama Project" validation.
var CanonicalProjects = map[string]string{
	"VERUA":  "Vertihome Serua",
	"LHL":    "Le Hauz Limo",
	"THP":    "The Hauz Pancoran Mas",
	"MAHABA": "Mahaba Village",
	"VERSAW": "Vertihauz Sawangan",
	"VERBUR": "Vertihauz Cibubur",
	"VERLIM": "Vertihauz Limo-3",
	"THC":    "The Hauz Cilodong",
	"ZHL":    "Z Hauz Limo",
	"LHC":    "Le Hauz Cibubur",
	"VERSER": "Vertihome Serpong",
	"THPJ":   "The Hauz Premiere",
}

// projectAliases maps spelling/phase variants seen in the raw data to their
// canonical code (after upper-casing, space removal and EXT-suffix stripping).
var projectAliases = map[string]string{
	"VERLIM3": "VERLIM",
	"VERLIM1": "VERLIM",
	"ZHL2":    "ZHL",
	"MAVIL":   "MAHABA",
	"MAVILL":  "MAHABA",
	"THPJ":    "THPJ",
}

// indoMonths maps Indonesian month names (and common misspellings such as
// "Febuari") to their month number.
var indoMonths = map[string]int{
	"januari": 1, "februari": 2, "pebruari": 2, "febuari": 2,
	"maret": 3, "april": 4, "mei": 5, "juni": 6, "juli": 7,
	"agustus": 8, "september": 9, "oktober": 10, "november": 11,
	"nopember": 11, "desember": 12,
}

var (
	nonDigit   = regexp.MustCompile(`\D`)
	multiSpace = regexp.MustCompile(`\s+`)
	idDateRe   = regexp.MustCompile(`^(\d{1,2})\s+([A-Za-z]+)\s+(\d{4})$`)
)

// NormalizePhone strips non-digits and forces the Indonesian 62 country prefix.
// Empty / unparseable input yields "".
func NormalizePhone(raw string) string {
	d := nonDigit.ReplaceAllString(raw, "")
	switch {
	case d == "":
		return ""
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	}
	return d
}

// ParseRupiah reads an Indonesian money string ("687.200.000") or a numeric
// cell into whole Rupiah. Returns (0, false) when there is no number.
func ParseRupiah(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	// A float-looking value from a numeric cell ("687200000" or "6.872e8").
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return int64(f), true
	}
	d := nonDigit.ReplaceAllString(raw, "")
	if d == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(d, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// ParseDate accepts the three date shapes found in the workbook:
//   - Indonesian text:  "25 Januari 2026", "21 Febuari 2026"
//   - ISO / datetime:   "2026-04-04", "2026-04-04 00:00:00"
//   - Excel serial:     "46116" (days since 1899-12-30)
//
// Non-date markers used in the Tgl Akad column ("Cancel", "In Proses", "-")
// return ok=false without being an error.
func ParseDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	if m := idDateRe.FindStringSubmatch(raw); m != nil {
		day, _ := strconv.Atoi(m[1])
		year, _ := strconv.Atoi(m[3])
		if mo, ok := indoMonths[strings.ToLower(m[2])]; ok {
			return time.Date(year, time.Month(mo), day, 0, 0, 0, 0, time.UTC), true
		}
		return time.Time{}, false
	}

	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", "01/02/2006", "02/01/2006"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}

	// Excel serial date (numbers > 1 with no separators).
	if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 59 {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.AddDate(0, 0, int(f)), true
	}
	return time.Time{}, false
}

// NormalizeProject canonicalizes a raw project label to its master code.
// known reports whether the result is in CanonicalProjects.
func NormalizeProject(raw string) (code string, known bool) {
	c := strings.ToUpper(strings.TrimSpace(raw))
	c = strings.ReplaceAll(c, " ", "")
	c = strings.TrimSuffix(c, "EXT")
	if alias, ok := projectAliases[c]; ok {
		c = alias
	}
	_, known = CanonicalProjects[c]
	return c, known
}

// collapseSpace trims and collapses internal whitespace runs to single spaces.
func collapseSpace(s string) string {
	return strings.TrimSpace(multiSpace.ReplaceAllString(s, " "))
}

// isNumericCell reports whether a raw cell value is a number (matching Excel's
// COUNT semantics — used to count Lead In Date cells). Dates read with
// RawCellValue come through as their numeric serial, so they count.
func isNumericCell(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// atoiLoose parses an integer that may be written as a float ("26.0") or carry
// thousand separators. ok=false when there is no number.
func atoiLoose(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f), true
	}
	d := nonDigit.ReplaceAllString(s, "")
	if d == "" {
		return 0, false
	}
	n, err := strconv.Atoi(d)
	if err != nil {
		return 0, false
	}
	return n, true
}
