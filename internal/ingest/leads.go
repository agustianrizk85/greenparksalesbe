package ingest

import (
	"sort"
	"strings"
	"time"

	"greenpark/sales/internal/domain"
)

// reasonsFrom turns reason-code counts into ordered domain.Reason rows (only
// codes that occurred), using the lead reason definitions.
func reasonsFrom(counts map[string]int) []domain.Reason {
	out := make([]domain.Reason, 0, len(leadReasonDefs))
	for _, def := range leadReasonDefs {
		if n := counts[def.Code]; n > 0 {
			def.Count = n
			out = append(out, def)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
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

// leadReasonDefs are the loss-reason codes derivable from a lead's follow-up
// status in MASTER DATA_LEADS. The leads stage captures Layer-1 losses
// (Leads → CV); deeper layers (CV→PV, PV→Booking) live in visit/sales data.
var leadReasonDefs = []domain.Reason{
	{Code: "ENG", Name: "Not Engaged", ID: "Belum Engage / Need Follow Up", Layer: "L1"},
	{Code: "UNR", Name: "Unreachable", ID: "Tidak Terhubung / No Respond", Layer: "L1"},
	{Code: "NQ", Name: "Not Qualified / Interest", ID: "Tidak Tertarik / Tidak Sesuai", Layer: "L1"},
	{Code: "WRN", Name: "Wrong Number", ID: "Nomor Salah / Tidak Valid", Layer: "L1"},
}

var leadReasonMeta = map[string]domain.ReasonMetaItem{
	"L1": {Stage: "Leads → CV", Target: "≥20% CV Rate"},
	"L2": {Stage: "CV → PV", Target: "≥70% PV Rate"},
	"L3": {Stage: "PV → Booking", Target: "≥30% Booking Rate"},
}

// leadReasonCode maps a follow-up stage value to a loss-reason code, or "" when
// the stage is a positive/progress status (Contacted, Visit, Booking, …).
func leadReasonCode(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	switch {
	case s == "":
		return ""
	case strings.Contains(s, "wrong number"):
		return "WRN"
	case strings.Contains(s, "unreachable"), strings.Contains(s, "no respon"), strings.Contains(s, "no respond"):
		return "UNR"
	case strings.Contains(s, "need follow"), strings.Contains(s, "follow up"), strings.Contains(s, "never follow"):
		return "ENG"
	case strings.Contains(s, "intrest"), strings.Contains(s, "interest"):
		return "NQ"
	default:
		return "" // contacted / visit / confirmed visit / booking / new / survey → not a loss
	}
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
	fu := []int{rs.col("follow up - 1", "follow up 1"), rs.col("follow up - 2"), rs.col("follow up - 3")}

	if cName < 0 {
		res.addIssue("Kolom Wajib", SevError, sheetLeads, 0, "kolom 'Name' tidak ditemukan")
		return nil
	}

	h := &res.Headline
	seen := map[string]int{} // normalized phone → first Excel row seen
	var leads, valid, cv, pv int
	reasonCount := map[string]int{} // reason code → count (from final follow-up stage)

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
			p = &projLeads{reasons: map[string]int{}}
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

		// Reason code (Opportunity Loss) — from this lead's FINAL follow-up stage.
		if len(stages) > 0 {
			if code := leadReasonCode(stages[len(stages)-1]); code != "" {
				reasonCount[code]++
				p.reasons[code]++
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

	// Reasons sourced from MASTER DATA_LEADS (per user).
	res.Preview.Reasons = reasonsFrom(reasonCount)
	res.Preview.ReasonMeta = leadReasonMeta

	std20, std70, std80 := 20, 70, 80
	return []domain.FunnelStage{
		{Key: "Leads", Value: int64(leads), Target: int64(leads), Owner: "Marketing", Std: nil},
		{Key: "Valid Leads", Value: int64(valid), Target: int64(leads) * 80 / 100, Owner: "Marketing", Std: &std80},
		{Key: "Confirmed Visit", Value: int64(cv), Target: int64(valid) * 20 / 100, Owner: "Sales", Std: &std20},
		{Key: "Project Visitor", Value: int64(pv), Target: int64(cv) * 70 / 100, Owner: "Sales", Std: &std70},
	}
}

// containsStage reports an exact (case-insensitive) stage match — used to keep
// "Visit" distinct from "Confirmed Visit".
func containsStage(stages []string, want string) bool {
	for _, s := range stages {
		if toLower(trim(s)) == want {
			return true
		}
	}
	return false
}

func lowerJoin(stages []string) string {
	out := ""
	for _, s := range stages {
		out += toLower(s) + " | "
	}
	return out
}
