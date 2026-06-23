package ingest

import (
	"sort"
	"strings"
	"time"

	"greenpark/sales/internal/domain"
)

// reasonAgg accumulates one (layer, code) bucket: its count and a capped sample
// of the lost prospects' identities (for the drill-down identity table).
type reasonAgg struct {
	count int
	leads []domain.ReasonLead
}

// add records one lost prospect under this bucket, capping the identity sample.
func (a *reasonAgg) add(l domain.ReasonLead) {
	a.count++
	if len(a.leads) < maxReasonLeads {
		a.leads = append(a.leads, l)
	}
}

// reasonKey composes the bucket key for a (layer, code) pair.
func reasonKey(layer, code string) string { return layer + "|" + code }

// reasonsFrom turns the (layer, code) buckets into ordered domain.Reason rows,
// carrying the capped identity sample for each. Rows are ordered by layer then
// by descending count so the L1/L2/L3 columns render in a stable order.
func reasonsFrom(buckets map[string]*reasonAgg) []domain.Reason {
	out := make([]domain.Reason, 0, len(buckets))
	for _, layer := range []string{"L1", "L2", "L3"} {
		for _, code := range reasonCodeOrder {
			agg := buckets[reasonKey(layer, code)]
			if agg == nil || agg.count == 0 {
				continue
			}
			def := reasonCodeDefs[code]
			out = append(out, domain.Reason{
				Code: code, Name: def.Name, ID: def.ID, Layer: layer,
				Count: agg.count, Leads: agg.leads,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Layer != out[j].Layer {
			return out[i].Layer < out[j].Layer
		}
		return out[i].Count > out[j].Count
	})
	return out
}

// Cleaning policy for the main funnel (agreed with the product owner):
//   - drop rows whose follow-up is "Wrong Number" / "Wrong Intrest"
//   - drop duplicate phone numbers (keep first occurrence)
//   - keep only Lead In Date within the reporting period
var (
	periodStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd   = time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC)
)

// inPeriod reports whether t falls within the reporting window.
func inPeriod(t time.Time) bool {
	return !t.Before(periodStart) && !t.After(periodEnd)
}

// maxReasonLeads caps how many identity rows are embedded per (layer, code)
// bucket, keeping the dashboard payload bounded while still backing the
// drill-down identity table (the Count keeps the exact total).
const maxReasonLeads = 500

// reasonDef holds the display labels for a loss-reason code. The same code can
// appear in any layer; the layer is derived from how far the lead progressed.
type reasonDef struct {
	Name string
	ID   string
}

// reasonCodeDefs maps each granular reason code (recorded in MASTER DATA_LEADS
// column L as "CODE - DESCRIPTION") to its display labels, grouped into the
// 3-layer Opportunity-Loss system (see reasonCodeLayer).
var reasonCodeDefs = map[string]reasonDef{
	// Layer 1 — Leads → CV
	"UNR": {Name: "Unreachable", ID: "Tidak Terhubung / No Respond"},
	"ENG": {Name: "Not Engaged", ID: "Belum Engage / No Schedule Locked"},
	"REJ": {Name: "Rejected", ID: "Ditolak / Tidak Valid"},
	"NQ":  {Name: "Not Qualified", ID: "Tidak Memenuhi Syarat"},
	// Layer 2 — CV → PV
	"SCH": {Name: "Schedule Conflict", ID: "Jadwal Bentrok"},
	"FOR": {Name: "Force Majeure", ID: "Force Majeure"},
	"COM": {Name: "Weak Commitment", ID: "Komitmen Lemah"},
	"INF": {Name: "Insufficient Information", ID: "Informasi Kurang"},
	"REM": {Name: "Reminder Failure", ID: "Gagal Reminder"},
	"EXP": {Name: "Expectation Not Set", ID: "Ekspektasi Tidak Selaras"},
	// Layer 3 — PV → P
	"FIN": {Name: "Financially Infeasible", ID: "Tidak Mampu Finansial"},
	"POL": {Name: "Policy Constraint", ID: "Kendala Kebijakan"},
	"PRD": {Name: "Product Mismatch", ID: "Produk Tidak Sesuai"},
	"NST": {Name: "No Next Step", ID: "Tidak Ada Langkah Lanjut"},
	"TIM": {Name: "Timing Delay", ID: "Penundaan Waktu"},
	"CMP": {Name: "Competitor Won", ID: "Kalah dari Kompetitor"},
	"DM":  {Name: "Decision Maker Missing", ID: "Pengambil Keputusan Tidak Ada"},
}

// reasonCodeLayer assigns each code to its loss layer (per the printed legend).
var reasonCodeLayer = map[string]string{
	"UNR": "L1", "ENG": "L1", "REJ": "L1", "NQ": "L1",
	"SCH": "L2", "FOR": "L2", "COM": "L2", "INF": "L2", "REM": "L2", "EXP": "L2",
	"FIN": "L3", "POL": "L3", "PRD": "L3", "NST": "L3", "TIM": "L3", "CMP": "L3", "DM": "L3",
}

// reasonCodeOrder fixes the within-layer ordering used when counts tie (legend
// order per layer).
var reasonCodeOrder = []string{
	"UNR", "ENG", "REJ", "NQ", // L1
	"SCH", "FOR", "COM", "INF", "REM", "EXP", // L2
	"FIN", "POL", "PRD", "NST", "TIM", "CMP", "DM", // L3
}

// leadReasonMeta names the three loss layers (Leads → CV → PV → P) shown as the
// Opportunity-Loss columns.
var leadReasonMeta = map[string]domain.ReasonMetaItem{
	"L1": {Stage: "Leads → CV", Target: "≥20% CV Rate"},
	"L2": {Stage: "CV → PV", Target: "≥70% PV Rate"},
	"L3": {Stage: "PV → P", Target: "≥30% Booking Rate"},
}

// parseReasonCode extracts the leading code from a MASTER DATA_LEADS column-L
// value ("UNR - UNREACHABLE" → UNR, "DM IG" → DM), returning "" when it is not
// one of the known reason codes.
func parseReasonCode(cell string) string {
	s := strings.ToUpper(strings.TrimSpace(cell))
	if s == "" {
		return ""
	}
	code := s
	if i := strings.IndexAny(s, " -"); i > 0 {
		code = strings.TrimSpace(s[:i])
	}
	if _, ok := reasonCodeLayer[code]; ok {
		return code
	}
	return ""
}

// findReasonCodeCol locates the (header-less) reason-code column by picking the
// column whose cells most often parse to a known reason code.
func findReasonCodeCol(rs rows) int {
	maxCol := len(rs.header)
	for _, r := range rs.data {
		if len(r) > maxCol {
			maxCol = len(r)
		}
	}
	best, bestCount := -1, 0
	for c := 0; c < maxCol; c++ {
		cnt := 0
		for i := range rs.data {
			if parseReasonCode(rs.cell(i, c)) != "" {
				cnt++
			}
		}
		if cnt > bestCount {
			bestCount, best = cnt, c
		}
	}
	return best
}

func isWrong(stages []string) bool {
	for _, s := range stages {
		if hasAny(s, "wrong number", "wrong intrest", "wrong interest") {
			return true
		}
	}
	return false
}

// mapLeads derives the Leads → Valid → CV → PV funnel from MASTER DATA_LEADS,
// applying the cleaning policy and recording the audit deltas + validations.
// It returns the four leads-side funnel stages; the purchaser stage is added by
// assemble() once sales is known.
func mapLeads(rs rows, res *Result) []domain.FunnelStage {
	cName := rs.col("name")
	cPhone := rs.col("phone")
	cDate := rs.col("lead in date", "date")
	cProj := rs.col("project")
	cReason := findReasonCodeCol(rs) // MASTER DATA_LEADS column L (header-less)
	fu := []int{rs.col("follow up - 1", "follow up 1"), rs.col("follow up - 2"), rs.col("follow up - 3")}

	if cName < 0 {
		res.addIssue("Kolom Wajib", SevError, sheetLeads, 0, "kolom 'Name' tidak ditemukan")
		return nil
	}

	h := &res.Headline
	seen := map[string]int{} // normalized phone → first Excel row seen
	var leads, valid, cv, pv int
	reasonBuckets := map[string]*reasonAgg{} // (layer|code) → count + capped identities

	// bump records one lost prospect into the (layer, code) bucket of m,
	// creating the bucket on first use.
	bump := func(m map[string]*reasonAgg, layer, code string, l domain.ReasonLead) {
		k := reasonKey(layer, code)
		agg := m[k]
		if agg == nil {
			agg = &reasonAgg{}
			m[k] = agg
		}
		agg.add(l)
	}

	stagesOf := func(i int) []string {
		out := make([]string, 0, 3)
		for _, c := range fu {
			if c < 0 {
				continue
			}
			if v := trim(rs.cell(i, c)); v != "" {
				out = append(out, v)
			}
		}
		return out
	}

	// drop records a discarded row (capped per reason) for the cleaning audit.
	dropCount := map[string]int{}
	drop := func(i int, reason, detail string) {
		if dropCount[reason] >= maxDroppedSamples {
			return
		}
		dropCount[reason]++
		date := ""
		if cDate >= 0 {
			date = trim(rs.cell(i, cDate))
			if t, ok := ParseDate(date); ok {
				date = t.Format("2006-01-02")
			}
		}
		proj := ""
		if cProj >= 0 {
			proj = trim(rs.cell(i, cProj))
		}
		res.Dropped = append(res.Dropped, DroppedRow{
			Row: i + 2, Reason: reason, Name: trim(rs.cell(i, cName)),
			Phone: NormalizePhone(rs.cell(i, cPhone)), Project: proj, Date: date, Detail: detail,
		})
	}

	// pl returns the per-project leads accumulator for a row's project.
	pl := func(i int) *projLeads {
		code := ""
		if cProj >= 0 {
			code, _ = NormalizeProject(rs.cell(i, cProj))
		}
		if code == "" {
			code = "—"
		}
		p, ok := res.leadsByProj[code]
		if !ok {
			p = &projLeads{reasons: map[string]*reasonAgg{}}
			res.leadsByProj[code] = p
		}
		return p
	}

	cFU1, cFU2, cFU3 := fu[0], fu[1], fu[2]
	for i := range rs.data {
		p := pl(i)
		// ---- Funnel counts: replicate the spreadsheet (DASHBOARD_CEO) formulas
		// EXACTLY so the dashboard matches the work file, column-by-column:
		//   Leads = COUNT(Lead In Date)        — numeric date cells
		//   Valid = COUNTIF(Follow up-1, "Contacted")
		//   CV    = COUNTIF(Follow up-2, "*Confirmed Visit*")   (contains)
		//   PV    = COUNTIF(Follow up-3, "Visit")
		if cDate >= 0 && isNumericCell(rs.cell(i, cDate)) {
			leads++
			p.leads++
		}
		if cFU1 >= 0 && strings.EqualFold(trim(rs.cell(i, cFU1)), "Contacted") {
			valid++
			p.valid++
		}
		if cFU2 >= 0 && contains(toLower(rs.cell(i, cFU2)), "confirmed visit") {
			cv++
			p.cv++
		}
		if cFU3 >= 0 && strings.EqualFold(trim(rs.cell(i, cFU3)), "Visit") {
			pv++
			p.pv++
		}

		// ---- Per-lead work (reason code + audit flags), keyed on a named row ----
		name := trim(rs.cell(i, cName))
		if name == "" {
			continue
		}
		h.LeadsRaw++
		stages := stagesOf(i)

		// Audit flags only (no cleaning / no exclusion).
		if isWrong(stages) {
			h.DroppedWrong++
			drop(i, "wrong", "status: "+strings.Join(stages, " / "))
		}
		if cDate >= 0 {
			if t, ok := ParseDate(rs.cell(i, cDate)); ok && (t.Before(periodStart) || t.After(periodEnd)) {
				h.DroppedPeriod++
				drop(i, "periode", "di luar Jan–Jun 2026")
			}
		}
		phone := NormalizePhone(rs.cell(i, cPhone))
		if phone != "" {
			if first, dup := seen[phone]; dup {
				h.DroppedDup++
				drop(i, "duplikat", "sama dengan baris "+itoa(first))
			} else {
				seen[phone] = i + 2
			}
		}

		// Reason code (Opportunity Loss) from MASTER DATA_LEADS column L
		// ("CODE - DESCRIPTION"), bucketed into the code's fixed layer.
		if cReason >= 0 {
			if code := parseReasonCode(rs.cell(i, cReason)); code != "" {
				layer := reasonCodeLayer[code]
				date := ""
				if cDate >= 0 {
					date = trim(rs.cell(i, cDate))
					if t, ok := ParseDate(date); ok {
						date = t.Format("2006-01-02")
					}
				}
				projName := ""
				if cProj >= 0 {
					projName = trim(rs.cell(i, cProj))
				}
				lead := domain.ReasonLead{
					Name: name, Phone: phone, Project: projName,
					Date: date, Status: trim(rs.cell(i, cReason)),
				}
				bump(reasonBuckets, layer, code, lead)
				bump(p.reasons, layer, code, lead)
			}
		}

		// project-name validation (warn, non-blocking)
		if cProj >= 0 {
			if p := trim(rs.cell(i, cProj)); p != "" {
				if _, known := NormalizeProject(p); !known {
					res.addIssue("Nama Project", SevWarning, sheetLeads, i+2,
						"project tidak dikenal: "+p)
				}
			}
		}
	}

	h.Leads, h.ValidLeads, h.CV, h.PV = leads, valid, cv, pv
	h.LeadsRaw = leads // semua data; tidak ada cleaning yang mengurangi

	// Reasons sourced from MASTER DATA_LEADS (per user), classified per layer.
	res.Preview.Reasons = reasonsFrom(reasonBuckets)
	res.Preview.ReasonMeta = leadReasonMeta

	std20, std70, std80 := 20, 70, 80
	return []domain.FunnelStage{
		{Key: "Leads", Value: int64(leads), Target: int64(leads), Owner: "Marketing", Std: nil},
		{Key: "Valid Leads", Value: int64(valid), Target: int64(leads) * 80 / 100, Owner: "Marketing", Std: &std80},
		{Key: "Confirmed Visit", Value: int64(cv), Target: int64(valid) * 20 / 100, Owner: "Sales", Std: &std20},
		{Key: "Project Visitor", Value: int64(pv), Target: int64(cv) * 70 / 100, Owner: "Sales", Std: &std70},
	}
}
