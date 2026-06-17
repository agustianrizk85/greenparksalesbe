// Package domain holds the core business entities of the Sales (penjualan)
// CEO War-Room dashboard. These types are the single source of truth for the
// data shape and carry no dependency on transport or storage concerns.
//
// Figures are sourced from the "DASHBOARD SALES_GREENPARK_2026" workbook
// (Q1 + Q2 2026). Monetary values are in full Rupiah unless noted otherwise.
package domain

// Exec is the executive snapshot: position against the 500-unit annual target.
type Exec struct {
	Target2026       int   `json:"target2026"`
	Booking          int   `json:"booking"`          // total unit dibooking
	Akad             int   `json:"akad"`             // selesai akad
	Proses           int   `json:"proses"`           // menuju akad / on proses
	Batal            int   `json:"batal"`            // gugur / cancel
	TotalPenjualan   int   `json:"totalPenjualan"`   // booking - batal
	RevenueAkad      int64 `json:"revenueAkad"`      // Rp terkonfirmasi akad
	PotentialRevenue int64 `json:"potentialRevenue"` // Rp potensi dari pipeline proses
	AdsSpent         int64 `json:"adsSpent"`
	AdsSpentQ1       int64 `json:"adsSpentQ1"`
	AdsSpentQ2       int64 `json:"adsSpentQ2"`
}

// MonthPoint is one month of the akad/booking trend.
type MonthPoint struct {
	M       string `json:"m"`
	Akad    int    `json:"akad"`
	Booking int    `json:"booking"`
}

// FunnelStage is one step of the leads → cash-in funnel.
type FunnelStage struct {
	Key     string `json:"key"`
	Value   int64  `json:"value"`
	Target  int64  `json:"target"`
	Owner   string `json:"owner"`
	Std     *int   `json:"std"`               // minimum conversion standard %, null when n/a
	IsMoney bool   `json:"isMoney,omitempty"` // value is a Rupiah amount
}

// Stock is the master-stock position of a project's units.
type Stock struct {
	Total   int `json:"total"`
	Closed  int `json:"closed"`
	Booking int `json:"booking"`
	Avail   int `json:"avail"`
}

// Project is the sales view of a project: bookings, akad, ads efficiency, stock.
// Cat is one of: utama | pendukung | pembenahan.
type Project struct {
	EntID  string  `json:"_id"`
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Total  int     `json:"total"`
	Akad   int     `json:"akad"`
	Proses int     `json:"proses"`
	Batal  int     `json:"batal"`
	Rev    int64   `json:"rev"` // revenue akad (Rp)
	Ads    int64   `json:"ads"` // ads spend (Rp)
	CPA    float64 `json:"cpa"` // cost per akad (Rp juta)
	Eff    string  `json:"eff"` // Excellent | Good | Fair | No Akad
	Cat    string  `json:"cat"` // utama | pendukung | pembenahan
	Stock  Stock   `json:"stock"`
}

// Channel is a sales source / lead channel with its booking and conversion.
type Channel struct {
	EntID string `json:"_id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Total int    `json:"total"`
	Akad  int    `json:"akad"`
	Conv  int    `json:"conv"`
}

// SalesRep is a single sales contributor's performance row.
type SalesRep struct {
	EntID   string `json:"_id"`
	Name    string `json:"name"`
	Role    string `json:"role"` // Internal | Agent
	Project string `json:"project"`
	Akad    int    `json:"akad"`
	Proses  int    `json:"proses"`
	Batal   int    `json:"batal"`
	Total   int    `json:"total"`
	Conv    int    `json:"conv"`
	Rev     int64  `json:"rev,omitempty"`
}

// ReasonMetaItem describes a reason-code layer: the stage and its target.
type ReasonMetaItem struct {
	Stage  string `json:"stage"`
	Target string `json:"target"`
}

// Reason is an opportunity-loss reason code in the 3-layer system.
type Reason struct {
	EntID string `json:"_id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	ID    string `json:"id"`    // Indonesian label
	Layer string `json:"layer"` // L1 | L2 | L3
	Count int    `json:"count"`
}

// Agent is an external agent/broker contributor.
type Agent struct {
	EntID   string `json:"_id"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Leads   string `json:"leads"`
	Akad    int    `json:"akad"`
	Total   int    `json:"total"`
	Conv    int    `json:"conv"`
	Status  string `json:"status"`
}

// MasterStock is the aggregate master-stock position across all units.
type MasterStock struct {
	Total   int     `json:"total"`
	Closed  int     `json:"closed"`
	Booking int     `json:"booking"`
	Hold    int     `json:"hold"`
	Avail   int     `json:"avail"`
	PctSold float64 `json:"pctSold"`
}

// EventAttributed is the event/walk-in channel performance proxy.
type EventAttributed struct {
	Name    string `json:"name"`
	Booking int    `json:"booking"`
	Akad    int    `json:"akad"`
	Conv    int    `json:"conv"`
}

// Events groups the attributed event metrics with a contextual note.
type Events struct {
	Attributed EventAttributed `json:"attributed"`
	Note       string          `json:"note"`
}

// Alert is a rule-based AI alert / daily action-plan item.
// Sev is one of: merah (critical) | kuning (risk) | hijau (opportunity).
type Alert struct {
	EntID    string `json:"_id"`
	Sev      string `json:"sev"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	PIC      string `json:"pic"`
	Deadline string `json:"deadline"`
	Action   string `json:"action"`
}

// KPI is a scorecard indicator with its actual value, target and owner.
type KPI struct {
	EntID       string  `json:"_id"`
	No          int     `json:"no"`
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	Target      float64 `json:"target"`
	Unit        string  `json:"unit"`
	Owner       string  `json:"owner"`
	Good        bool    `json:"good"`
	LowerBetter bool    `json:"lowerBetter,omitempty"`
}

// Summary holds the executive KPIs derived from the rest of the data set.
type Summary struct {
	Target2026       int     `json:"target2026"`
	Akad             int     `json:"akad"`
	Booking          int     `json:"booking"`
	Proses           int     `json:"proses"`
	Batal            int     `json:"batal"`
	Achievement      float64 `json:"achievement"`      // akad / target %
	GapToTarget      int     `json:"gapToTarget"`      // target - akad (units)
	PipelineActive   int     `json:"pipelineActive"`   // akad + proses
	CancelRate       float64 `json:"cancelRate"`       // batal / booking %
	BookingToAkad    float64 `json:"bookingToAkad"`    // akad / booking %
	CashIn           int64   `json:"cashIn"`           // revenue akad (Rp)
	PotentialRevenue int64   `json:"potentialRevenue"` // pipeline potential (Rp)
	AdsSpent         int64   `json:"adsSpent"`         // total ads spend (Rp)
	AvgCostPerAkad   float64 `json:"avgCostPerAkad"`   // ads / akad (Rp juta)
	TotalProjects    int     `json:"totalProjects"`
	TotalSalesReps   int     `json:"totalSalesReps"`
	StockSold        float64 `json:"stockSold"` // master stock % sold
	Status           string  `json:"status"`    // on-track | risk | off-track
}

// ProjectView is the per-project slice of the dashboard, so the project filter
// can make every panel (funnel, channels, sales, reasons, agents, trend) follow
// the selected project using the same logic as the global view.
type ProjectView struct {
	Exec     Exec          `json:"exec"`
	Funnel   []FunnelStage `json:"funnel"`
	Channels []Channel     `json:"channels"`
	Sales    []SalesRep    `json:"sales"`
	Reasons  []Reason      `json:"reasons"`
	Agents   []Agent       `json:"agents"`
	Monthly  []MonthPoint  `json:"monthly"`
	Events   Events        `json:"events"`
}

// Dashboard is the full payload consumed by the front-end in a single call.
type Dashboard struct {
	Period     string                    `json:"period"`
	Updated    string                    `json:"updated"`
	Exec       Exec                      `json:"exec"`
	Monthly    []MonthPoint              `json:"monthly"`
	Funnel     []FunnelStage             `json:"funnel"`
	Projects   []Project                 `json:"projects"`
	Channels   []Channel                 `json:"channels"`
	Sales      []SalesRep                `json:"sales"`
	ReasonMeta map[string]ReasonMetaItem `json:"reasonMeta"`
	Reasons    []Reason                  `json:"reasons"`
	Agents     []Agent                   `json:"agents"`
	Stock      MasterStock               `json:"stock"`
	Events     Events                    `json:"events"`
	Alerts     []Alert                   `json:"alerts"`
	KPIs       []KPI                     `json:"kpis"`
	Summary    Summary                   `json:"summary"`
	ByProject  map[string]ProjectView    `json:"byProject,omitempty"`
}

// Entity is implemented by every CRUD collection element. The synthetic _id is
// the stable handle the master-data admin uses to update/delete a row,
// independent of any editable business field (code, name, …).
type Entity interface {
	GetID() string
}

func (p Project) GetID() string  { return p.EntID }
func (s SalesRep) GetID() string { return s.EntID }
func (c Channel) GetID() string  { return c.EntID }
func (r Reason) GetID() string   { return r.EntID }
func (a Agent) GetID() string    { return a.EntID }
func (a Alert) GetID() string    { return a.EntID }
func (k KPI) GetID() string      { return k.EntID }

// Role enumerates the access levels for a dashboard user.
type Role string

const (
	RoleAdmin  Role = "admin"  // full master-data access
	RoleViewer Role = "viewer" // read-only dashboard access
)

// User is a dashboard account. Password material is never serialised to clients
// (json:"-"); it only lives in the persisted store.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Role         Role   `json:"role"`
	PasswordHash string `json:"-"`
	Salt         string `json:"-"`
}
