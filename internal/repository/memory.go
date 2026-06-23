package repository

import (
	"fmt"
	"strings"
	"sync"

	"greenpark/sales/internal/domain"
)

// fileRepository is a mutex-guarded SalesRepository. The full state is held in
// memory for fast reads and flushed to its persister (file or DB) on every write.
type fileRepository struct {
	mu  sync.RWMutex
	p   persister
	st  *state
	rev int64 // data revision, bumped on every write (for FE realtime refresh)
}

// NewRepository returns a SalesRepository persisted to the given JSON file path.
// An empty path keeps everything in memory only (handy for tests).
func NewRepository(path string) (SalesRepository, error) {
	return newRepository(filePersister{path: path})
}

// NewPostgresRepository returns a SalesRepository that persists the whole-state
// snapshot to a single PostgreSQL row.
func NewPostgresRepository(dsn string) (SalesRepository, error) {
	p, err := newPGPersister(dsn)
	if err != nil {
		return nil, err
	}
	return newRepository(p)
}

func newRepository(p persister) (SalesRepository, error) {
	st, err := p.load()
	if err != nil {
		return nil, err
	}
	return &fileRepository{p: p, st: st}, nil
}

// persist flushes the current state. Callers must hold the write lock.
func (r *fileRepository) persist() error {
	r.rev++
	return r.p.save(r.st)
}

// Revision returns the current data revision. The front-end polls this cheaply
// and reloads the dashboard only when it changes (realtime auto-refresh).
func (r *fileRepository) Revision() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rev
}

/* ---------------------------- reads ---------------------------- */

func (r *fileRepository) Period() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.st.Period
}

func (r *fileRepository) Updated() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.st.Updated
}

func (r *fileRepository) Exec() domain.Exec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.st.Exec
}

func (r *fileRepository) Monthly() []domain.MonthPoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Monthly)
}

func (r *fileRepository) Funnel() []domain.FunnelStage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Funnel)
}

func (r *fileRepository) Projects() []domain.Project {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Projects)
}

func (r *fileRepository) ProjectByCode(code string) (domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	code = strings.ToUpper(code)
	for _, p := range r.st.Projects {
		if p.Code == code {
			return p, nil
		}
	}
	return domain.Project{}, ErrNotFound
}

func (r *fileRepository) Channels() []domain.Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Channels)
}

func (r *fileRepository) Sales() []domain.SalesRep {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Sales)
}

func (r *fileRepository) ReasonMeta() map[string]domain.ReasonMetaItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]domain.ReasonMetaItem, len(r.st.ReasonMeta))
	for k, v := range r.st.ReasonMeta {
		out[k] = v
	}
	return out
}

func (r *fileRepository) Reasons() []domain.Reason {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Reasons)
}

func (r *fileRepository) Agents() []domain.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Agents)
}

func (r *fileRepository) Stock() domain.MasterStock {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.st.Stock
}

func (r *fileRepository) Events() domain.Events {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.st.Events
}

func (r *fileRepository) Alerts() []domain.Alert {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.Alerts)
}

func (r *fileRepository) KPIs() []domain.KPI {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.KPIs)
}

func (r *fileRepository) ByProject() map[string]domain.ProjectView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]domain.ProjectView, len(r.st.ByProject))
	for k, v := range r.st.ByProject {
		out[k] = v
	}
	return out
}

func (r *fileRepository) SaleRows() []domain.SaleRow {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return clone(r.st.SaleRows)
}

/* ---------------------------- singleton writes ---------------------------- */

func (r *fileRepository) SetMeta(period, updated string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Period = period
	r.st.Updated = updated
	return r.persist()
}

func (r *fileRepository) SetExec(e domain.Exec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Exec = e
	return r.persist()
}

func (r *fileRepository) SetStock(s domain.MasterStock) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Stock = s
	return r.persist()
}

func (r *fileRepository) SetEvents(ev domain.Events) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Events = ev
	return r.persist()
}

func (r *fileRepository) SetFunnel(f []domain.FunnelStage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Funnel = f
	return r.persist()
}

func (r *fileRepository) SetMonthly(m []domain.MonthPoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.Monthly = m
	return r.persist()
}

func (r *fileRepository) SetReasonMeta(rm map[string]domain.ReasonMetaItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.st.ReasonMeta = rm
	return r.persist()
}

/* ---------------------------- collection writes ---------------------------- */

func (r *fileRepository) SaveProject(p domain.Project) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.EntID == "" {
		p.EntID = newID("prj")
	}
	r.st.Projects = upsertEntity(r.st.Projects, p)
	return p, r.persist()
}

func (r *fileRepository) DeleteProject(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Projects, id)
	r.st.Projects = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveSalesRep(s domain.SalesRep) (domain.SalesRep, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.EntID == "" {
		s.EntID = newID("sal")
	}
	r.st.Sales = upsertEntity(r.st.Sales, s)
	return s, r.persist()
}

func (r *fileRepository) DeleteSalesRep(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Sales, id)
	r.st.Sales = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveChannel(c domain.Channel) (domain.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.EntID == "" {
		c.EntID = newID("chn")
	}
	r.st.Channels = upsertEntity(r.st.Channels, c)
	return c, r.persist()
}

func (r *fileRepository) DeleteChannel(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Channels, id)
	r.st.Channels = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveReason(rs domain.Reason) (domain.Reason, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rs.EntID == "" {
		rs.EntID = newID("rsn")
	}
	r.st.Reasons = upsertEntity(r.st.Reasons, rs)
	return rs, r.persist()
}

func (r *fileRepository) DeleteReason(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Reasons, id)
	r.st.Reasons = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveAgent(a domain.Agent) (domain.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.EntID == "" {
		a.EntID = newID("agt")
	}
	r.st.Agents = upsertEntity(r.st.Agents, a)
	return a, r.persist()
}

func (r *fileRepository) DeleteAgent(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Agents, id)
	r.st.Agents = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveAlert(a domain.Alert) (domain.Alert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.EntID == "" {
		a.EntID = newID("alt")
	}
	r.st.Alerts = upsertEntity(r.st.Alerts, a)
	return a, r.persist()
}

func (r *fileRepository) DeleteAlert(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.Alerts, id)
	r.st.Alerts = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

func (r *fileRepository) SaveKPI(k domain.KPI) (domain.KPI, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if k.EntID == "" {
		k.EntID = newID("kpi")
	}
	r.st.KPIs = upsertEntity(r.st.KPIs, k)
	return k, r.persist()
}

func (r *fileRepository) DeleteKPI(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	next, ok := deleteEntity(r.st.KPIs, id)
	r.st.KPIs = next
	if !ok {
		return false, nil
	}
	return true, r.persist()
}

/* ---------------------------- import ---------------------------- */

// maxImportHistory caps how many history entries (with undo snapshots) we keep.
const maxImportHistory = 20

// takeSnapshot captures the full dashboard state for one-step rollback. Callers
// must hold the write lock.
func (r *fileRepository) takeSnapshot() *snapshot {
	return &snapshot{
		Period: r.st.Period, Updated: r.st.Updated, Exec: r.st.Exec,
		Monthly: clone(r.st.Monthly), Funnel: clone(r.st.Funnel),
		Projects: clone(r.st.Projects), Channels: clone(r.st.Channels),
		Sales: clone(r.st.Sales), Stock: r.st.Stock, Events: r.st.Events,
		ReasonMeta: cloneReasonMeta(r.st.ReasonMeta), Reasons: clone(r.st.Reasons),
		Agents: clone(r.st.Agents), Alerts: clone(r.st.Alerts), KPIs: clone(r.st.KPIs),
		ByProject: cloneByProject(r.st.ByProject), SaleRows: clone(r.st.SaleRows),
	}
}

func cloneByProject(m map[string]domain.ProjectView) map[string]domain.ProjectView {
	if m == nil {
		return nil
	}
	out := make(map[string]domain.ProjectView, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// restoreSnapshot writes a snapshot back over the live state. Callers must hold
// the write lock.
func (r *fileRepository) restoreSnapshot(s *snapshot) {
	r.st.Period, r.st.Updated, r.st.Exec = s.Period, s.Updated, s.Exec
	r.st.Monthly, r.st.Funnel = s.Monthly, s.Funnel
	r.st.Projects, r.st.Channels, r.st.Sales = s.Projects, s.Channels, s.Sales
	r.st.Stock, r.st.Events = s.Stock, s.Events
	r.st.ReasonMeta, r.st.Reasons = s.ReasonMeta, s.Reasons
	r.st.Agents, r.st.Alerts, r.st.KPIs = s.Agents, s.Alerts, s.KPIs
	r.st.ByProject = s.ByProject
	r.st.SaleRows = s.SaleRows
}

func cloneReasonMeta(m map[string]domain.ReasonMetaItem) map[string]domain.ReasonMetaItem {
	if m == nil {
		return nil
	}
	out := make(map[string]domain.ReasonMetaItem, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (r *fileRepository) ApplyImport(in ImportInput) (domain.ImportRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Capture the pre-import state for rollback.
	prev := r.takeSnapshot()

	// Assign stable synthetic ids to the incoming collections.
	projects := make([]domain.Project, len(in.Projects))
	for i, p := range in.Projects {
		p.EntID = "prj-" + strings.ToLower(p.Code)
		projects[i] = p
	}
	channels := make([]domain.Channel, len(in.Channels))
	for i, c := range in.Channels {
		c.EntID = "chn-" + strings.ToLower(c.Code)
		channels[i] = c
	}
	reps := make([]domain.SalesRep, len(in.Sales))
	for i, s := range in.Sales {
		s.EntID = newID("sal")
		reps[i] = s
	}
	reasons := make([]domain.Reason, len(in.Reasons))
	for i, rs := range in.Reasons {
		rs.EntID = "rsn-" + strings.ToLower(rs.Code)
		reasons[i] = rs
	}
	agents := make([]domain.Agent, len(in.Agents))
	for i, a := range in.Agents {
		a.EntID = newID("agt")
		agents[i] = a
	}
	alerts := make([]domain.Alert, len(in.Alerts))
	for i, a := range in.Alerts {
		a.EntID = newID("alt")
		alerts[i] = a
	}
	kpis := make([]domain.KPI, len(in.KPIs))
	for i, k := range in.KPIs {
		k.EntID = fmt.Sprintf("kpi-%02d", k.No)
		kpis[i] = k
	}

	r.st.Period = in.Period
	r.st.Updated = in.Updated
	r.st.Exec = in.Exec
	r.st.Monthly = in.Monthly
	r.st.Funnel = in.Funnel
	r.st.Projects = projects
	r.st.Channels = channels
	r.st.Sales = reps
	r.st.Stock = in.Stock
	r.st.Events = in.Events
	// Reasons/ReasonMeta come from the REASON CODE ANALYSIS sheet when present;
	// leave the existing ones untouched if the upload carried none. Agents/Alerts/
	// KPIs are derived from DATA PENJUALAN, so always replace them.
	if len(reasons) > 0 {
		r.st.Reasons = reasons
	}
	if len(in.ReasonMeta) > 0 {
		r.st.ReasonMeta = in.ReasonMeta
	}
	r.st.Agents = agents
	r.st.Alerts = alerts
	r.st.KPIs = kpis
	r.st.ByProject = in.ByProject
	r.st.SaleRows = in.SaleRows

	entry := importEntry{
		ID: in.ID, Time: in.Time, Filename: in.Filename, By: in.By,
		Summary: in.Summary, Prev: prev,
	}
	// Newest first; cap history length.
	r.st.Imports = append([]importEntry{entry}, r.st.Imports...)
	if len(r.st.Imports) > maxImportHistory {
		r.st.Imports = r.st.Imports[:maxImportHistory]
	}
	return entry.toRecord(), r.persist()
}

func (r *fileRepository) ImportHistory() []domain.ImportRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.ImportRecord, len(r.st.Imports))
	for i, e := range r.st.Imports {
		out[i] = e.toRecord()
	}
	return out
}

func (r *fileRepository) Rollback(id string) (domain.ImportRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.st.Imports {
		e := &r.st.Imports[i]
		if e.ID != id {
			continue
		}
		if e.RolledBack || e.Prev == nil {
			return domain.ImportRecord{}, ErrNotFound
		}
		r.restoreSnapshot(e.Prev)
		e.RolledBack = true
		e.Prev = nil // snapshot consumed
		return e.toRecord(), r.persist()
	}
	return domain.ImportRecord{}, ErrNotFound
}

// ResetData clears all dashboard data (upload-derived figures AND the manual
// collections), returning the store to its empty state while keeping the annual
// target. The pre-reset state is snapshotted into the history so the wipe can be
// rolled back in one step.
func (r *fileRepository) ResetData(by, when string) (domain.ImportRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prev := r.takeSnapshot()

	r.st.Period = "Belum ada data — silakan upload Excel"
	r.st.Updated = "—"
	r.st.Exec = domain.Exec{Target2026: prev.Exec.Target2026}
	r.st.Monthly = []domain.MonthPoint{}
	r.st.Funnel = []domain.FunnelStage{}
	r.st.Projects = []domain.Project{}
	r.st.Channels = []domain.Channel{}
	r.st.Sales = []domain.SalesRep{}
	r.st.Stock = domain.MasterStock{}
	r.st.Events = domain.Events{}
	r.st.ReasonMeta = map[string]domain.ReasonMetaItem{}
	r.st.Reasons = []domain.Reason{}
	r.st.Agents = []domain.Agent{}
	r.st.Alerts = []domain.Alert{}
	r.st.KPIs = []domain.KPI{}
	r.st.ByProject = map[string]domain.ProjectView{}
	r.st.SaleRows = []domain.SaleRow{}

	entry := importEntry{
		ID:       newID("rst"),
		Time:     when,
		Filename: "(hapus semua data)",
		By:       by,
		Prev:     prev,
	}
	r.st.Imports = append([]importEntry{entry}, r.st.Imports...)
	if len(r.st.Imports) > maxImportHistory {
		r.st.Imports = r.st.Imports[:maxImportHistory]
	}
	return entry.toRecord(), r.persist()
}

/* ---------------------------- users ---------------------------- */

func (r *fileRepository) Users() []domain.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.User, len(r.st.Users))
	for i, u := range r.st.Users {
		out[i] = u.toDomain()
	}
	return out
}

func (r *fileRepository) UserByUsername(username string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	username = strings.ToLower(strings.TrimSpace(username))
	for _, u := range r.st.Users {
		if strings.ToLower(u.Username) == username {
			return u.toDomain(), nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (r *fileRepository) UserByID(id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.st.Users {
		if u.ID == id {
			return u.toDomain(), nil
		}
	}
	return domain.User{}, ErrNotFound
}
