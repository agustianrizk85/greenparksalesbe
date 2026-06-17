package ingest

import "greenpark/sales/internal/domain"

// mapStock parses the per-project summary block of MASTER STOCK UNIT. That sheet
// is a free-form dashboard layout, so we locate the header row by content
// ("KODE … PROJECT") and read fixed offsets from the KODE column:
//
//	KODE | NAMA | TOTAL | TERJUAL(CLOSED) | BOOKING(PROSES) | HOLD | SISA(AVAILABLE)
func mapStock(raw [][]string, res *Result) {
	kCol, hdrRow := findStockHeader(raw)
	if kCol < 0 {
		res.addIssue("Master Stock", SevWarning, sheetStock, 0, "header KODE PROJECT tidak ditemukan")
		return
	}

	var agg domain.MasterStock
	for r := hdrRow + 1; r < len(raw); r++ {
		row := raw[r]
		// End of the per-project block: the TOTAL/SISA recap row, whose label
		// may sit in any column.
		if rowHasAny(row, "total keseluruhan", "sisa stock") {
			break
		}
		kode := cellAt(row, kCol)
		if kode == "" {
			continue
		}
		// The left summary block numbers each project (NO column = kCol-1). The
		// unrelated GP recap block below it uses "GP 1"/"GP 2" instead, so a
		// non-numeric NO marks a row we must not aggregate.
		if _, ok := atoiLoose(cellAt(row, kCol-1)); !ok {
			continue
		}
		code, _ := NormalizeProject(kode)
		total, okT := atoiLoose(cellAt(row, kCol+2))
		if !okT {
			continue // not a data row
		}
		closed, _ := atoiLoose(cellAt(row, kCol+3))
		booking, _ := atoiLoose(cellAt(row, kCol+4))
		hold, _ := atoiLoose(cellAt(row, kCol+5))
		avail, _ := atoiLoose(cellAt(row, kCol+6))

		res.stockByProj[code] = domain.Stock{Total: total, Closed: closed, Booking: booking, Avail: avail}
		agg.Total += total
		agg.Closed += closed
		agg.Booking += booking
		agg.Hold += hold
		agg.Avail += avail
	}
	if agg.Total > 0 {
		agg.PctSold = round1(float64(agg.Closed) / float64(agg.Total) * 100)
	}
	res.Preview.Stock = agg
}

// findStockHeader returns the column index of the KODE cell and its row index.
func findStockHeader(raw [][]string) (col, row int) {
	for r := range raw {
		for c, cell := range raw[r] {
			if hasAny(cell, "kode") && hasAny(cell, "project") {
				return c, r
			}
		}
	}
	return -1, -1
}

// rowHasAny reports whether any cell in the row contains one of the substrings.
func rowHasAny(row []string, subs ...string) bool {
	for _, c := range row {
		if hasAny(c, subs...) {
			return true
		}
	}
	return false
}

func cellAt(row []string, c int) string {
	if c < 0 || c >= len(row) {
		return ""
	}
	return trim(row[c])
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
