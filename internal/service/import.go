package service

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/ingest"
	"greenpark/sales/internal/repository"
)

// indoMonths renders Updated stamps in Indonesian.
var indoMonths = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// PreviewImport parses an uploaded workbook and returns the validated, mapped
// preview WITHOUT touching the live dashboard.
func (s *salesService) PreviewImport(r io.Reader) (*ingest.Result, error) {
	return ingest.RunReader(r)
}

// PreviewSheets parses sheets fetched from Google Sheets without touching the
// live dashboard.
func (s *salesService) PreviewSheets(data map[string][][]string) (*ingest.Result, error) {
	return ingest.RunSheets(data)
}

// ApproveImport parses the workbook, applies the mapped data to the store,
// records a rollback snapshot + history entry, and returns the new record.
func (s *salesService) ApproveImport(r io.Reader, filename, by string) (domain.ImportRecord, error) {
	res, err := ingest.RunReader(r)
	if err != nil {
		return domain.ImportRecord{}, err
	}
	return s.applyResult(res, filename, by)
}

// ApproveSheets applies a Google-Sheets-sourced import to the store.
func (s *salesService) ApproveSheets(data map[string][][]string, filename, by string) (domain.ImportRecord, error) {
	res, err := ingest.RunSheets(data)
	if err != nil {
		return domain.ImportRecord{}, err
	}
	return s.applyResult(res, filename, by)
}

// applyResult turns an ingest Result into a persisted import.
func (s *salesService) applyResult(res *ingest.Result, filename, by string) (domain.ImportRecord, error) {
	now := time.Now()
	d := res.Preview
	h := res.Headline

	in := repository.ImportInput{
		ID:       newImportID(),
		Time:     now.Format(time.RFC3339),
		Filename: filename,
		By:       by,
		Summary: domain.ImportSummary{
			Leads: h.Leads, ValidLeads: h.ValidLeads, CV: h.CV, PV: h.PV,
			Booking: h.Booking, Akad: h.Akad, Proses: h.Proses, Batal: h.Batal,
			CashIn: h.CashIn, Issues: len(res.Issues),
		},
		Period:     d.Period,
		Updated:    stampDate(now),
		Exec:       d.Exec,
		Monthly:    d.Monthly,
		Funnel:     d.Funnel,
		Projects:   d.Projects,
		Channels:   d.Channels,
		Sales:      d.Sales,
		Stock:      d.Stock,
		Events:     d.Events,
		Reasons:    d.Reasons,
		ReasonMeta: d.ReasonMeta,
		Agents:     d.Agents,
		Alerts:     d.Alerts,
		KPIs:       d.KPIs,
		ByProject:  d.ByProject,
		SaleRows:   d.SaleRows,
	}
	return s.repo.ApplyImport(in)
}

// ResetData clears all dashboard data back to empty (reversible via history).
func (s *salesService) ResetData(by string) (domain.ImportRecord, error) {
	return s.repo.ResetData(by, time.Now().Format(time.RFC3339))
}

// Revision returns the data revision counter (bumped on every write).
func (s *salesService) Revision() int64 { return s.repo.Revision() }

// ImportHistory returns the recorded import history (newest first).
func (s *salesService) ImportHistory() []domain.ImportRecord { return s.repo.ImportHistory() }

// RollbackImport restores the dashboard to its state before the given import.
func (s *salesService) RollbackImport(id string) (domain.ImportRecord, error) {
	return s.repo.Rollback(id)
}

func stampDate(t time.Time) string {
	return time.Time(t).Format("2") + " " + indoMonths[int(t.Month())] + " " + time.Time(t).Format("2006")
}

func newImportID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "imp-0000000000"
	}
	return "imp-" + hex.EncodeToString(b)
}
