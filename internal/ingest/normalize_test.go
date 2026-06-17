package ingest

import (
	"testing"
	"time"
)

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"81290607725":    "6281290607725",
		"081286479731":   "6281286479731",
		"6281315512924":  "6281315512924",
		"0852-7143-939":  "628527143939", // hyphens stripped, leading 0→62
		"":               "",
		"abc":            "",
		"(021) 555 1234": "62215551234",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRupiah(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"687.200.000", 687200000, true},
		{"687200000", 687200000, true},
		{"Rp 827.600.000", 827600000, true},
		{"", 0, false},
		{"-", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseRupiah(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseRupiah(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	want := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"4 April 2026", "2026-04-04", "2026-04-04 00:00:00"} {
		got, ok := ParseDate(in)
		if !ok || !got.Equal(want) {
			t.Errorf("ParseDate(%q) = (%v,%v), want %v", in, got, ok, want)
		}
	}
	// Indonesian misspelling "Febuari".
	if got, ok := ParseDate("21 Febuari 2026"); !ok || got.Month() != time.February || got.Day() != 21 {
		t.Errorf("ParseDate(Febuari) = (%v,%v)", got, ok)
	}
	// Non-date markers from the Tgl Akad column.
	for _, in := range []string{"Cancel", "In Proses", "-", ""} {
		if _, ok := ParseDate(in); ok {
			t.Errorf("ParseDate(%q) parsed unexpectedly", in)
		}
	}
}

func TestNormalizeProject(t *testing.T) {
	cases := map[string]struct {
		code  string
		known bool
	}{
		"VERLIM3":      {"VERLIM", true},
		"VERLIM 3 EXT": {"VERLIM", true},
		"VERBUR EXT":   {"VERBUR", true},
		"ZHL 2":        {"ZHL", true},
		"MAVILL":       {"MAHABA", true},
		"THP J":        {"THPJ", true},
		"THP":          {"THP", true},
		"CMGP":         {"CMGP", false},
	}
	for in, want := range cases {
		code, known := NormalizeProject(in)
		if code != want.code || known != want.known {
			t.Errorf("NormalizeProject(%q) = (%q,%v), want (%q,%v)", in, code, known, want.code, want.known)
		}
	}
}
