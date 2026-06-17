package ingest

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// workbook abstracts the data source so the engine can read either an uploaded
// XLSX file or a live Google Sheet (fetched via the Sheets API) with identical
// downstream logic.
type workbook interface {
	// rawRows returns every row of a sheet as strings, with numbers/dates kept
	// as their raw stored value (no display formatting).
	rawRows(name string) ([][]string, error)
	// sheets lists the available sheet names.
	sheets() []string
}

// xlsxBook adapts an excelize workbook to the workbook interface.
type xlsxBook struct{ f *excelize.File }

func (b xlsxBook) sheets() []string { return b.f.GetSheetList() }

func (b xlsxBook) rawRows(name string) ([][]string, error) {
	if idx, err := b.f.GetSheetIndex(name); err != nil || idx < 0 {
		return nil, fmt.Errorf("sheet %q not found", name)
	}
	// RawCellValue keeps numbers/dates/phones as their stored value instead of
	// the display-formatted string. Mapping does its own parsing.
	r, err := b.f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("read sheet %q: %w", name, err)
	}
	return r, nil
}

// sheetsBook adapts a set of pre-fetched sheets (e.g. from the Google Sheets
// API, read with UNFORMATTED_VALUE) to the workbook interface.
type sheetsBook struct{ data map[string][][]string }

func (b sheetsBook) sheets() []string {
	out := make([]string, 0, len(b.data))
	for k := range b.data {
		out = append(out, k)
	}
	return out
}

func (b sheetsBook) rawRows(name string) ([][]string, error) {
	if r, ok := b.data[name]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("sheet %q not found", name)
}
