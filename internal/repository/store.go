package repository

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"greenpark/sales/internal/domain"
)

// state is the full persisted snapshot of the dashboard data plus the user
// accounts. It is the on-disk JSON shape; the derived Summary is never stored.
type state struct {
	Period     string                           `json:"period"`
	Updated    string                           `json:"updated"`
	Exec       domain.Exec                      `json:"exec"`
	Monthly    []domain.MonthPoint              `json:"monthly"`
	Funnel     []domain.FunnelStage             `json:"funnel"`
	Projects   []domain.Project                 `json:"projects"`
	Channels   []domain.Channel                 `json:"channels"`
	Sales      []domain.SalesRep                `json:"sales"`
	ReasonMeta map[string]domain.ReasonMetaItem `json:"reasonMeta"`
	Reasons    []domain.Reason                  `json:"reasons"`
	Agents     []domain.Agent                   `json:"agents"`
	Stock      domain.MasterStock               `json:"stock"`
	Events     domain.Events                    `json:"events"`
	Alerts     []domain.Alert                   `json:"alerts"`
	KPIs       []domain.KPI                     `json:"kpis"`
	ByProject  map[string]domain.ProjectView    `json:"byProject,omitempty"`
	Users      []storeUser                      `json:"users"`
	Imports    []importEntry                    `json:"imports,omitempty"`
}

// snapshot captures the full dashboard state so an import or a reset can be
// rolled back in one step. An import only mutates the upload-derived slice, but
// snapshotting everything keeps rollback correct for the "delete all data"
// reset too (which clears the manual collections as well).
type snapshot struct {
	Period     string                           `json:"period"`
	Updated    string                           `json:"updated"`
	Exec       domain.Exec                      `json:"exec"`
	Monthly    []domain.MonthPoint              `json:"monthly"`
	Funnel     []domain.FunnelStage             `json:"funnel"`
	Projects   []domain.Project                 `json:"projects"`
	Channels   []domain.Channel                 `json:"channels"`
	Sales      []domain.SalesRep                `json:"sales"`
	Stock      domain.MasterStock               `json:"stock"`
	Events     domain.Events                    `json:"events"`
	ReasonMeta map[string]domain.ReasonMetaItem `json:"reasonMeta"`
	Reasons    []domain.Reason                  `json:"reasons"`
	Agents     []domain.Agent                   `json:"agents"`
	Alerts     []domain.Alert                   `json:"alerts"`
	KPIs       []domain.KPI                     `json:"kpis"`
	ByProject  map[string]domain.ProjectView    `json:"byProject,omitempty"`
}

// importEntry is a persisted import-history row. Prev holds the pre-import state
// for one-step rollback; it is cleared once an import has been rolled back.
type importEntry struct {
	ID         string               `json:"id"`
	Time       string               `json:"time"`
	Filename   string               `json:"filename"`
	By         string               `json:"by"`
	Summary    domain.ImportSummary `json:"summary"`
	RolledBack bool                 `json:"rolledBack"`
	Prev       *snapshot            `json:"prev,omitempty"`
}

func (e importEntry) toRecord() domain.ImportRecord {
	return domain.ImportRecord{
		ID: e.ID, Time: e.Time, Filename: e.Filename, By: e.By,
		Summary: e.Summary, RolledBack: e.RolledBack, CanRollback: e.Prev != nil && !e.RolledBack,
	}
}

// storeUser is the persisted user shape. Unlike domain.User (which hides
// password material from API responses via json:"-"), this type DOES serialise
// the salt and hash so accounts survive a restart. It never leaves the store.
type storeUser struct {
	ID           string      `json:"id"`
	Username     string      `json:"username"`
	Name         string      `json:"name"`
	Role         domain.Role `json:"role"`
	PasswordHash string      `json:"passwordHash"`
	Salt         string      `json:"salt"`
}

func (u storeUser) toDomain() domain.User {
	return domain.User{
		ID:           u.ID,
		Username:     u.Username,
		Name:         u.Name,
		Role:         u.Role,
		PasswordHash: u.PasswordHash,
		Salt:         u.Salt,
	}
}

// seedState builds a fresh store: an EMPTY dashboard data set plus the default
// accounts. Every figure is left at its zero value (collections nil/empty,
// singletons zeroed) so the dashboard reads empty until the first workbook is
// imported. Only the annual akad target is pre-set, as it is configuration.
func seedState() *state {
	return &state{
		Period:  "Belum ada data — silakan upload Excel",
		Updated: "—",
		Exec:    domain.Exec{Target2026: defaultTarget2026},
		// Non-nil empty slices so the JSON contract stays [] (never null), which
		// the front-end maps over directly.
		Monthly:    []domain.MonthPoint{},
		Funnel:     []domain.FunnelStage{},
		Projects:   []domain.Project{},
		Channels:   []domain.Channel{},
		Sales:      []domain.SalesRep{},
		ReasonMeta: map[string]domain.ReasonMetaItem{},
		Reasons:    []domain.Reason{},
		Agents:     []domain.Agent{},
		Alerts:     []domain.Alert{},
		KPIs:       []domain.KPI{},
		Users:      seedUsers(),
	}
}

// load reads the state from disk; if the file is missing it seeds a fresh state
// and writes it. An empty path means in-memory only (used by tests).
func load(path string) (*state, error) {
	if path == "" {
		return seedState(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s := seedState()
			if err := save(path, s); err != nil {
				return nil, err
			}
			return s, nil
		}
		return nil, err
	}
	s := &state{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	return s, nil
}

// save atomically writes the state to disk (write temp + rename).
func save(path string, s *state) error {
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// newID returns a short, collision-resistant identifier with the given prefix.
func newID(prefix string) string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should not fail; fall back to a fixed marker.
		return prefix + "-0000000000"
	}
	return prefix + "-" + hex.EncodeToString(b)
}

/* ---------- generic CRUD helpers over []T where T implements domain.Entity ---------- */

func findEntity[T domain.Entity](xs []T, id string) (T, bool) {
	for _, x := range xs {
		if x.GetID() == id {
			return x, true
		}
	}
	var zero T
	return zero, false
}

// upsertEntity replaces the element whose id matches, otherwise appends it.
func upsertEntity[T domain.Entity](xs []T, item T) []T {
	for i, x := range xs {
		if x.GetID() == item.GetID() {
			xs[i] = item
			return xs
		}
	}
	return append(xs, item)
}

// deleteEntity removes the element with the given id, reporting whether it existed.
func deleteEntity[T domain.Entity](xs []T, id string) ([]T, bool) {
	for i, x := range xs {
		if x.GetID() == id {
			return append(xs[:i:i], xs[i+1:]...), true
		}
	}
	return xs, false
}

// clone returns a shallow copy of a slice (so reads never alias the stored slice).
func clone[T any](xs []T) []T {
	if xs == nil {
		return nil
	}
	out := make([]T, len(xs))
	copy(out, xs)
	return out
}

// persister abstracts where the whole-state snapshot lives (JSON file or DB row).
// The repository logic is identical regardless of backend.
type persister interface {
	load() (*state, error)
	save(*state) error
}

// filePersister stores the state as an atomic JSON file on disk.
type filePersister struct{ path string }

func (f filePersister) load() (*state, error) { return load(f.path) }
func (f filePersister) save(st *state) error  { return save(f.path, st) }
