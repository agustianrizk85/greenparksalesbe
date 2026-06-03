package repository

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Users      []storeUser                      `json:"users"`
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

// seedState builds the default data set (seed figures + admin/viewer accounts)
// and assigns a stable synthetic _id to every collection row.
func seedState() *state {
	s := &state{
		Period:     "Q1 + Q2 2026 · April–Juni",
		Updated:    "17 April 2026",
		Exec:       seedExec(),
		Monthly:    seedMonthly(),
		Funnel:     seedFunnel(),
		Projects:   seedProjects(),
		Channels:   seedChannels(),
		Sales:      seedSales(),
		ReasonMeta: seedReasonMeta(),
		Reasons:    seedReasons(),
		Agents:     seedAgents(),
		Stock:      seedStock(),
		Events:     seedEvents(),
		Alerts:     seedAlerts(),
		KPIs:       seedKPIs(),
		Users:      seedUsers(),
	}
	assignSeedIDs(s)
	return s
}

// assignSeedIDs gives each seeded row a readable, deterministic _id derived from
// its natural key (code/name/no) or position.
func assignSeedIDs(s *state) {
	for i := range s.Projects {
		s.Projects[i].EntID = "prj-" + strings.ToLower(s.Projects[i].Code)
	}
	for i := range s.Channels {
		s.Channels[i].EntID = "chn-" + strings.ToLower(s.Channels[i].Code)
	}
	for i := range s.Reasons {
		s.Reasons[i].EntID = "rsn-" + strings.ToLower(s.Reasons[i].Code)
	}
	for i := range s.Sales {
		s.Sales[i].EntID = fmt.Sprintf("sal-%02d", i+1)
	}
	for i := range s.Agents {
		s.Agents[i].EntID = fmt.Sprintf("agt-%02d", i+1)
	}
	for i := range s.Alerts {
		s.Alerts[i].EntID = fmt.Sprintf("alt-%02d", i+1)
	}
	for i := range s.KPIs {
		s.KPIs[i].EntID = fmt.Sprintf("kpi-%02d", s.KPIs[i].No)
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
