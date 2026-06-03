package repository

import (
	"strings"
	"sync"

	"greenpark/sales/internal/domain"
)

// fileRepository is a mutex-guarded, file-backed SalesRepository. The full state
// is held in memory for fast reads and flushed to disk on every write.
type fileRepository struct {
	mu   sync.RWMutex
	path string
	st   *state
}

// NewRepository returns a SalesRepository persisted to the given JSON file path.
// An empty path keeps everything in memory only (handy for tests).
func NewRepository(path string) (SalesRepository, error) {
	st, err := load(path)
	if err != nil {
		return nil, err
	}
	return &fileRepository{path: path, st: st}, nil
}

// persist flushes the current state to disk. Callers must hold the write lock.
func (r *fileRepository) persist() error { return save(r.path, r.st) }

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
