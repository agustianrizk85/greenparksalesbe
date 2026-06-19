// Package service holds the business logic of the Sales dashboard. It composes
// the repository data and computes the derived executive summary, keeping
// transport handlers thin. Write use-cases delegate to the repository (which
// persists), so master-data edits flow straight back into the dashboard read.
package service

import (
	"io"
	"math"

	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/ingest"
	"greenpark/sales/internal/repository"
)

// SalesService exposes the read and write use-cases of the dashboard.
type SalesService interface {
	// reads
	Dashboard() domain.Dashboard
	Summary() domain.Summary
	Exec() domain.Exec
	Funnel() []domain.FunnelStage
	Projects() []domain.Project
	ProjectByCode(code string) (domain.Project, error)
	Channels() []domain.Channel
	Sales() []domain.SalesRep
	Reasons() []domain.Reason
	Agents() []domain.Agent
	Alerts() []domain.Alert
	KPIs() []domain.KPI

	// singleton writes
	SetMeta(period, updated string) error
	SetExec(domain.Exec) error
	SetStock(domain.MasterStock) error
	SetEvents(domain.Events) error
	SetFunnel([]domain.FunnelStage) error
	SetMonthly([]domain.MonthPoint) error
	SetReasonMeta(map[string]domain.ReasonMetaItem) error

	// collection writes
	SaveProject(domain.Project) (domain.Project, error)
	DeleteProject(id string) (bool, error)
	SaveSalesRep(domain.SalesRep) (domain.SalesRep, error)
	DeleteSalesRep(id string) (bool, error)
	SaveChannel(domain.Channel) (domain.Channel, error)
	DeleteChannel(id string) (bool, error)
	SaveReason(domain.Reason) (domain.Reason, error)
	DeleteReason(id string) (bool, error)
	SaveAgent(domain.Agent) (domain.Agent, error)
	DeleteAgent(id string) (bool, error)
	SaveAlert(domain.Alert) (domain.Alert, error)
	DeleteAlert(id string) (bool, error)
	SaveKPI(domain.KPI) (domain.KPI, error)
	DeleteKPI(id string) (bool, error)

	// import (upload pipeline)
	PreviewImport(r io.Reader) (*ingest.Result, error)
	ApproveImport(r io.Reader, filename, by string) (domain.ImportRecord, error)
	PreviewSheets(data map[string][][]string) (*ingest.Result, error)
	ApproveSheets(data map[string][][]string, filename, by string) (domain.ImportRecord, error)
	ImportHistory() []domain.ImportRecord
	RollbackImport(id string) (domain.ImportRecord, error)
	ResetData(by string) (domain.ImportRecord, error)
	Revision() int64
}

type salesService struct {
	repo repository.SalesRepository
}

// New returns a SalesService backed by the given repository.
func New(repo repository.SalesRepository) SalesService {
	return &salesService{repo: repo}
}

// categorizeAkad tiers a project by akad volume. No master classification
// exists in the source data, so the category is derived from performance:
// Mesin Utama (akad ≥ 10), Pembenahan (akad ≤ 1), Pendukung (in between).
func categorizeAkad(akad int) string {
	switch {
	case akad >= 10:
		return "utama"
	case akad <= 1:
		return "pembenahan"
	default:
		return "pendukung"
	}
}

// Dashboard assembles the full payload including the derived summary.
func (s *salesService) Dashboard() domain.Dashboard {
	// Re-derive the project category from akad on every read, so snapshots
	// stored before categorisation existed (all "pendukung") are corrected
	// without a re-import. (Copy to avoid mutating the repo's backing slice.)
	src := s.repo.Projects()
	projects := make([]domain.Project, len(src))
	copy(projects, src)
	for i := range projects {
		projects[i].Cat = categorizeAkad(projects[i].Akad)
	}
	return domain.Dashboard{
		Period:     s.repo.Period(),
		Updated:    s.repo.Updated(),
		Exec:       s.repo.Exec(),
		Monthly:    s.repo.Monthly(),
		Funnel:     s.repo.Funnel(),
		Projects:   projects,
		Channels:   s.repo.Channels(),
		Sales:      s.repo.Sales(),
		ReasonMeta: s.repo.ReasonMeta(),
		Reasons:    s.repo.Reasons(),
		Agents:     s.repo.Agents(),
		Stock:      s.repo.Stock(),
		Events:     s.repo.Events(),
		Alerts:     s.repo.Alerts(),
		KPIs:       s.repo.KPIs(),
		Summary:    s.Summary(),
		ByProject:  s.repo.ByProject(),
	}
}

// Summary computes the executive KPIs from the exec snapshot, master stock and
// the project / sales-rep counts.
func (s *salesService) Summary() domain.Summary {
	e := s.repo.Exec()
	stock := s.repo.Stock()
	projects := s.repo.Projects()
	reps := s.repo.Sales()

	achievement := 0.0
	cancelRate := 0.0
	bookingToAkad := 0.0
	if e.Target2026 > 0 {
		achievement = round1(float64(e.Akad) / float64(e.Target2026) * 100)
	}
	if e.Booking > 0 {
		cancelRate = round1(float64(e.Batal) / float64(e.Booking) * 100)
		bookingToAkad = round1(float64(e.Akad) / float64(e.Booking) * 100)
	}
	avgCostPerAkad := 0.0
	if e.Akad > 0 {
		avgCostPerAkad = round1(float64(e.AdsSpent) / float64(e.Akad) / 1_000_000)
	}

	return domain.Summary{
		Target2026:       e.Target2026,
		Akad:             e.Akad,
		Booking:          e.Booking,
		Proses:           e.Proses,
		Batal:            e.Batal,
		Achievement:      achievement,
		GapToTarget:      e.Target2026 - e.Akad,
		PipelineActive:   e.Akad + e.Proses,
		CancelRate:       cancelRate,
		BookingToAkad:    bookingToAkad,
		CashIn:           e.RevenueAkad,
		PotentialRevenue: e.PotentialRevenue,
		AdsSpent:         e.AdsSpent,
		AvgCostPerAkad:   avgCostPerAkad,
		TotalProjects:    len(projects),
		TotalSalesReps:   len(reps),
		StockSold:        stock.PctSold,
		Status:           statusFor(bookingToAkad, achievement),
	}
}

// statusFor derives a qualitative health label from achievement / conversion.
func statusFor(bookingToAkad, achievement float64) string {
	switch {
	case achievement >= 60 && bookingToAkad >= 70:
		return "on-track"
	case achievement >= 25 || bookingToAkad >= 50:
		return "risk"
	default:
		return "off-track"
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

/* ---- reads ---- */

func (s *salesService) Exec() domain.Exec            { return s.repo.Exec() }
func (s *salesService) Funnel() []domain.FunnelStage { return s.repo.Funnel() }
func (s *salesService) Projects() []domain.Project   { return s.repo.Projects() }
func (s *salesService) ProjectByCode(code string) (domain.Project, error) {
	return s.repo.ProjectByCode(code)
}
func (s *salesService) Channels() []domain.Channel { return s.repo.Channels() }
func (s *salesService) Sales() []domain.SalesRep   { return s.repo.Sales() }
func (s *salesService) Reasons() []domain.Reason   { return s.repo.Reasons() }
func (s *salesService) Agents() []domain.Agent     { return s.repo.Agents() }
func (s *salesService) Alerts() []domain.Alert     { return s.repo.Alerts() }
func (s *salesService) KPIs() []domain.KPI         { return s.repo.KPIs() }

/* ---- singleton writes ---- */

func (s *salesService) SetMeta(period, updated string) error   { return s.repo.SetMeta(period, updated) }
func (s *salesService) SetExec(e domain.Exec) error            { return s.repo.SetExec(e) }
func (s *salesService) SetStock(m domain.MasterStock) error    { return s.repo.SetStock(m) }
func (s *salesService) SetEvents(ev domain.Events) error       { return s.repo.SetEvents(ev) }
func (s *salesService) SetFunnel(f []domain.FunnelStage) error { return s.repo.SetFunnel(f) }
func (s *salesService) SetMonthly(m []domain.MonthPoint) error { return s.repo.SetMonthly(m) }
func (s *salesService) SetReasonMeta(rm map[string]domain.ReasonMetaItem) error {
	return s.repo.SetReasonMeta(rm)
}

/* ---- collection writes ---- */

func (s *salesService) SaveProject(p domain.Project) (domain.Project, error) {
	return s.repo.SaveProject(p)
}
func (s *salesService) DeleteProject(id string) (bool, error) { return s.repo.DeleteProject(id) }
func (s *salesService) SaveSalesRep(x domain.SalesRep) (domain.SalesRep, error) {
	return s.repo.SaveSalesRep(x)
}
func (s *salesService) DeleteSalesRep(id string) (bool, error) { return s.repo.DeleteSalesRep(id) }
func (s *salesService) SaveChannel(c domain.Channel) (domain.Channel, error) {
	return s.repo.SaveChannel(c)
}
func (s *salesService) DeleteChannel(id string) (bool, error) { return s.repo.DeleteChannel(id) }
func (s *salesService) SaveReason(r domain.Reason) (domain.Reason, error) {
	return s.repo.SaveReason(r)
}
func (s *salesService) DeleteReason(id string) (bool, error) { return s.repo.DeleteReason(id) }
func (s *salesService) SaveAgent(a domain.Agent) (domain.Agent, error) {
	return s.repo.SaveAgent(a)
}
func (s *salesService) DeleteAgent(id string) (bool, error) { return s.repo.DeleteAgent(id) }
func (s *salesService) SaveAlert(a domain.Alert) (domain.Alert, error) {
	return s.repo.SaveAlert(a)
}
func (s *salesService) DeleteAlert(id string) (bool, error) { return s.repo.DeleteAlert(id) }
func (s *salesService) SaveKPI(k domain.KPI) (domain.KPI, error) {
	return s.repo.SaveKPI(k)
}
func (s *salesService) DeleteKPI(id string) (bool, error) { return s.repo.DeleteKPI(id) }
