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

	// ---- users (auth) ----
	Users() []domain.User
	UserByUsername(username string) (domain.User, error)
	UserByID(id string) (domain.User, error)
}
