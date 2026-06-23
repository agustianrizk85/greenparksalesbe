package ingest

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"greenpark/sales/internal/domain"
)

func toLower(s string) string     { return strings.ToLower(s) }
func contains(s, sub string) bool { return strings.Contains(s, sub) }
func trim(s string) string        { return strings.TrimSpace(s) }
func itoa(n int) string           { return strconv.Itoa(n) }

// orDash renders an empty value as "-" for readable issue messages.
func orDash(s string) string {
	if trim(s) == "" {
		return "-"
	}
	return s
}

func hasAny(s string, subs ...string) bool {
	ls := strings.ToLower(s)
	for _, x := range subs {
		if strings.Contains(ls, x) {
			return true
		}
	}
	return false
}

// Sheet names in the source workbook.
const (
	sheetPenjualan = "DATA PENJUALAN"
	sheetLeads     = "MASTER DATA_LEADS"
	sheetVisitor   = "MASTER DATA_VISITOR"
	sheetStock     = "MASTER STOCK UNIT"
	sheetAds       = "META ADS INPUT"
)

// Severity classifies a validation issue.
type Severity string

const (
	SevError   Severity = "error"   // blocks approval
	SevWarning Severity = "warning" // surfaced, does not block
	SevInfo    Severity = "info"    // informational
)

// Issue is one validation finding tied to a sheet/row when applicable.
type Issue struct {
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Sheet    string   `json:"sheet"`
	Row      int      `json:"row,omitempty"` // 1-based Excel row, 0 = sheet-level
	Message  string   `json:"message"`
}

// Headline is the at-a-glance preview the admin approves. It mirrors the main
// funnel and sales figures, plus the cleaning deltas for the leads stage.
type Headline struct {
	// Main funnel (from MASTER DATA_LEADS, cleaned)
	Leads      int `json:"leads"`
	ValidLeads int `json:"validLeads"`
	CV         int `json:"cv"`
	PV         int `json:"pv"`

	// Sales (from DATA PENJUALAN)
	Purchaser int   `json:"purchaser"` // Sumber=LEADS & status != Batal (BRD)
	Booking   int   `json:"booking"`   // semua transaksi termasuk Batal (BRD)
	Akad      int   `json:"akad"`
	Proses    int   `json:"proses"`
	Batal     int   `json:"batal"`
	CashIn    int64 `json:"cashIn"`

	// Cleaning audit for the leads stage
	LeadsRaw       int `json:"leadsRaw"`
	DroppedWrong   int `json:"droppedWrong"`
	DroppedDup     int `json:"droppedDup"`
	DroppedPeriod  int `json:"droppedPeriod"`
	VisitorWalkIns int `json:"visitorWalkIns"` // MASTER DATA_VISITOR, event panel only
}

// DroppedRow is one MASTER DATA_LEADS row removed during cleaning, with the
// reason it was discarded. Used to let the admin audit the cleaning step.
type DroppedRow struct {
	Row     int    `json:"row"`    // 1-based Excel row
	Reason  string `json:"reason"` // wrong | duplikat | periode
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Project string `json:"project"`
	Date    string `json:"date"`
	Detail  string `json:"detail"`
}

// maxDroppedSamples caps how many dropped rows are returned per reason so the
// preview payload stays bounded (the headline keeps the exact totals).
const maxDroppedSamples = 1000

// Result is the full outcome of an ingest run: the headline, the validation
// report, the row counts read per sheet, the dropped-row audit, and the mapped
// dashboard preview.
type Result struct {
	Headline Headline          `json:"headline"`
	Issues   []Issue           `json:"issues"`
	RowsRead map[string]int    `json:"rowsRead"`
	Dropped  []DroppedRow      `json:"dropped"`
	Preview  *domain.Dashboard `json:"preview"`

	// stockByProj holds the per-project stock position parsed from MASTER STOCK
	// UNIT, keyed by canonical code; consumed by assemble().
	stockByProj map[string]domain.Stock
	// leadsByProj holds the per-project funnel + reason counts from
	// MASTER DATA_LEADS, keyed by canonical code; consumed by assemble().
	leadsByProj map[string]*projLeads
}

// projLeads is one project's leads-side funnel and reason distribution. Reasons
// are keyed by (layer, code) — see reasonKey — and carry capped identities.
type projLeads struct {
	leads, valid, cv, pv int
	reasons              map[string]*reasonAgg
}

// addIssue appends a finding to the result.
func (r *Result) addIssue(check string, sev Severity, sheet string, row int, msg string) {
	r.Issues = append(r.Issues, Issue{Check: check, Severity: sev, Sheet: sheet, Row: row, Message: msg})
}

// ErrorCount returns the number of blocking (error-severity) issues.
func (r *Result) ErrorCount() int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == SevError {
			n++
		}
	}
	return n
}

// rows is a loaded sheet: header row plus data rows, already trimmed to the used
// rectangle by excelize. Each row is a []string of cell values.
type rows struct {
	name   string
	header []string
	data   [][]string
}

// col returns the index of the first header containing any of subs
// (case-insensitive), or -1 when none match.
func (rs rows) col(subs ...string) int {
	for i, h := range rs.header {
		lh := toLower(h)
		for _, s := range subs {
			if contains(lh, toLower(s)) {
				return i
			}
		}
	}
	return -1
}

// colAll returns every header index matching any of subs (case-insensitive).
// Used to pick the right column when a header appears more than once (the
// workbook has two "Platform" columns: raw and cleaned).
func (rs rows) colAll(subs ...string) []int {
	var out []int
	for i, h := range rs.header {
		lh := toLower(h)
		for _, s := range subs {
			if contains(lh, toLower(s)) {
				out = append(out, i)
				break
			}
		}
	}
	return out
}

// cell safely returns column c of data row i (0-based), or "" when out of range.
func (rs rows) cell(i, c int) string {
	if i < 0 || i >= len(rs.data) {
		return ""
	}
	row := rs.data[i]
	if c < 0 || c >= len(row) {
		return ""
	}
	return row[c]
}

// Run parses the workbook at path and returns the validated, mapped preview.
// A parse-level failure (missing file, unreadable workbook, missing required
// sheet) is returned as an error; data-level problems become Issues.
func Run(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	return run(xlsxBook{f})
}

// RunReader is like Run but reads the workbook from r (e.g. an HTTP upload).
func RunReader(r io.Reader) (*Result, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	return run(xlsxBook{f})
}

// RunSheets runs the engine over sheets fetched from the Google Sheets API
// (keyed by sheet title, values read as UNFORMATTED_VALUE / strings).
func RunSheets(data map[string][][]string) (*Result, error) {
	return run(sheetsBook{data})
}

func run(wb workbook) (*Result, error) {
	res := &Result{RowsRead: map[string]int{}, Preview: &domain.Dashboard{}, stockByProj: map[string]domain.Stock{}, leadsByProj: map[string]*projLeads{}}

	// --- required sheets ---
	penjualan, err := loadSheet(wb, sheetPenjualan, 1)
	if err != nil {
		return nil, err
	}
	leads, err := loadSheet(wb, sheetLeads, 1)
	if err != nil {
		return nil, err
	}

	// --- optional sheets (warn, do not fail) ---
	visitor, vErr := loadSheet(wb, sheetVisitor, 1)
	if vErr != nil {
		res.addIssue("Sheet", SevWarning, sheetVisitor, 0, vErr.Error())
	}
	ads, aErr := loadSheet(wb, sheetAds, 3)
	if aErr != nil {
		res.addIssue("Sheet", SevWarning, sheetAds, 0, aErr.Error())
	}
	stockRows, sErr := loadRaw(wb, sheetStock)
	if sErr != nil {
		res.addIssue("Sheet", SevWarning, sheetStock, 0, sErr.Error())
	}

	res.RowsRead[sheetPenjualan] = len(penjualan.data)
	res.RowsRead[sheetLeads] = len(leads.data)
	if vErr == nil {
		res.RowsRead[sheetVisitor] = len(visitor.data)
	}

	// --- map each domain ---
	funnel := mapLeads(leads, res)
	sales := mapPenjualan(penjualan, res)
	if vErr == nil {
		mapVisitor(visitor, res)
	}
	if aErr == nil {
		mapAds(ads, sales, res)
	}
	if sErr == nil {
		mapStock(stockRows, res)
	}
	// Reason codes are derived from MASTER DATA_LEADS inside mapLeads (per user).

	assemble(res, funnel, sales)
	return res, nil
}

// loadSheet reads a sheet whose header is on row headerRow (1-based) and returns
// the header plus the data rows beneath it.
func loadSheet(wb workbook, name string, headerRow int) (rows, error) {
	raw, err := loadRaw(wb, name)
	if err != nil {
		return rows{}, err
	}
	if len(raw) < headerRow {
		return rows{name: name}, nil
	}
	hdr := raw[headerRow-1]
	for i := range hdr {
		hdr[i] = collapseSpace(hdr[i])
	}
	return rows{name: name, header: hdr, data: raw[headerRow:]}, nil
}

// loadRaw returns every row of a sheet as strings, resolving the sheet name
// case/space-insensitively (Google tabs / Excel may carry stray spacing).
func loadRaw(wb workbook, name string) ([][]string, error) {
	if r, err := wb.rawRows(name); err == nil {
		return r, nil
	}
	wantN := strings.ToUpper(strings.TrimSpace(name))
	for _, s := range wb.sheets() {
		if strings.ToUpper(strings.TrimSpace(s)) == wantN {
			return wb.rawRows(s)
		}
	}
	return nil, fmt.Errorf("sheet %q not found", name)
}
