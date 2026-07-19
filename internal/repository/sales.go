// Package repository defines storage access for the Sales dashboard and ships
// a file-backed, in-memory implementation seeded with representative data.
// Writes are mutex-guarded and persisted to a JSON file so master-data edits
// survive restarts. Swapping in a database-backed store only requires
// satisfying the SalesRepository interface.
package repository

import (
	"errors"

	"greenpark/sales/internal/domain"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("resource not found")

// SalesRepository is the persistence boundary for the dashboard data set.
type SalesRepository interface {
	// ---- reads ----
	Period() string
	Updated() string
	Exec() domain.Exec
	Monthly() []domain.MonthPoint
	Funnel() []domain.FunnelStage
	Projects() []domain.Project
	ProjectByCode(code string) (domain.Project, error)
	Channels() []domain.Channel
	Sales() []domain.SalesRep
	ReasonMeta() map[string]domain.ReasonMetaItem
	Reasons() []domain.Reason
	Agents() []domain.Agent
	Stock() domain.MasterStock
	Events() domain.Events
	Alerts() []domain.Alert
	KPIs() []domain.KPI
	ByProject() map[string]domain.ProjectView
	SaleRows() []domain.SaleRow

	// ---- singleton writes ----
	SetMeta(period, updated string) error
	SetExec(domain.Exec) error
	SetStock(domain.MasterStock) error
	SetEvents(domain.Events) error
	SetFunnel([]domain.FunnelStage) error
	SetMonthly([]domain.MonthPoint) error
	SetReasonMeta(map[string]domain.ReasonMetaItem) error

	// ---- collection writes (Save = create when _id empty, else update) ----
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

	// ---- import (upload pipeline) ----
	// ApplyImport replaces the upload-derived slice of the dashboard with the
	// mapped data, recording an undo snapshot + history entry. It leaves
	// Reasons/Agents/Alerts/KPIs/ReasonMeta untouched.
	ApplyImport(in ImportInput) (domain.ImportRecord, error)
	ImportHistory() []domain.ImportRecord
	Rollback(id string) (domain.ImportRecord, error)
	// ResetData clears all dashboard data back to empty (reversible via history).
	ResetData(by, when string) (domain.ImportRecord, error)

	// ---- konsumen screening ----
	// Questions are a singleton set the admin (Kadep) replaces as a whole.
	ScreeningQuestions() []domain.ScreeningQuestion
	SetScreeningQuestions([]domain.ScreeningQuestion) error
	// Submissions are appended per completed screening (newest first, capped).
	ScreeningSubmissions() []domain.ScreeningSubmission
	SaveScreeningSubmission(domain.ScreeningSubmission) (domain.ScreeningSubmission, error)
	DeleteScreeningSubmission(id string) (bool, error)

	// Revision returns a counter that bumps on every write (FE realtime refresh).
	Revision() int64

	// ---- users (auth) ----
	Users() []domain.User
	UserByUsername(username string) (domain.User, error)
	UserByID(id string) (domain.User, error)
}

// ImportInput is the payload an approved upload applies to the store.
type ImportInput struct {
	ID       string
	Time     string
	Filename string
	By       string
	Summary  domain.ImportSummary

	Period     string
	Updated    string
	Exec       domain.Exec
	Monthly    []domain.MonthPoint
	Funnel     []domain.FunnelStage
	Projects   []domain.Project
	Channels   []domain.Channel
	Sales      []domain.SalesRep
	Stock      domain.MasterStock
	Events     domain.Events
	Reasons    []domain.Reason
	ReasonMeta map[string]domain.ReasonMetaItem
	Agents     []domain.Agent
	Alerts     []domain.Alert
	KPIs       []domain.KPI
	ByProject  map[string]domain.ProjectView
	SaleRows   []domain.SaleRow
}
