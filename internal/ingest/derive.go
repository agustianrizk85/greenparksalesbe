package ingest

import (
	"fmt"
	"sort"

	"greenpark/sales/internal/domain"
)

// grouped formats an integer with dot thousand separators (Indonesian style).
func grouped(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// pctOf returns a/b*100 (rounded to 1 dp), 0 when b==0.
func pctOf(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return round1(float64(a) / float64(b) * 100)
}

/* ---------------- Panel 9: Agent & Event ---------------- */

func buildAgents(sd *salesData) []domain.Agent { return agentsFromMap(sd.agentOrder, sd.agents) }

func agentsFromMap(order []string, agents map[string]*repAgg) []domain.Agent {
	out := make([]domain.Agent, 0, len(order))
	for _, name := range order {
		a := agents[name]
		total := a.akad + a.proses + a.batal
		conv := int(pctOf(a.akad, total))
		out = append(out, domain.Agent{
			Name: name, Project: a.project, Leads: "—",
			Akad: a.akad, Total: total, Conv: conv, Status: agentStatus(a.akad, conv),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Akad > out[j].Akad })
	return out
}

func agentStatus(akad, conv int) string {
	switch {
	case akad == 0:
		return "Need Reactivation"
	case conv >= 60:
		return "Active Productive"
	default:
		return "Active Low Conv"
	}
}

func buildEvents(sd *salesData) domain.Events {
	return domain.Events{
		Attributed: domain.EventAttributed{
			Name:    "Walkin",
			Booking: sd.eventBooking,
			Akad:    sd.eventAkad,
			Conv:    int(pctOf(sd.eventAkad, sd.eventBooking)),
		},
		Note: "Sumber: DATA PENJUALAN (Sumber = Walk-in/Undangan) sebagai proxy. Detail visit di MASTER DATA_VISITOR.",
	}
}

/* ---------------- KPI Scorecard ---------------- */

func buildKPIs(d *domain.Dashboard, sd *salesData, h Headline) []domain.KPI {
	agentAkad := 0
	for _, a := range sd.agents {
		agentAkad += a.akad
	}
	costPerBooking := 0.0
	if h.Booking > 0 {
		costPerBooking = round1(float64(d.Exec.AdsSpent) / float64(h.Booking) / 1_000_000)
	}
	mk := func(no int, name string, val, target float64, unit, owner string, good, lower bool) domain.KPI {
		return domain.KPI{No: no, Name: name, Value: val, Target: target, Unit: unit, Owner: owner, Good: good, LowerBetter: lower}
	}
	validRate := pctOf(h.ValidLeads, h.Leads)
	leadsToCV := pctOf(h.CV, h.ValidLeads)
	cvToPV := pctOf(h.PV, h.CV)
	pvToPurch := pctOf(h.Purchaser, h.PV)
	bookingToAkad := pctOf(h.Akad, h.Booking)
	agentContrib := pctOf(agentAkad, h.Akad)

	return []domain.KPI{
		mk(1, "Valid Leads Rate", validRate, 80, "%", "Marketing", validRate >= 80, false),
		mk(2, "Leads → CV Rate", leadsToCV, 20, "%", "Sales", leadsToCV >= 20, false),
		mk(3, "CV → PV Rate", cvToPV, 70, "%", "Sales", cvToPV >= 70, false),
		mk(4, "PV → Purchaser Rate", pvToPurch, 30, "%", "Sales", pvToPurch >= 30, false),
		mk(5, "Booking → Akad Rate", bookingToAkad, 70, "%", "Sales/KPR", bookingToAkad >= 70, false),
		mk(6, "Cash-In Achievement", bookingToAkad, 100, "%", "Finance/KPR", bookingToAkad >= 100, false),
		mk(7, "Agent Booking Contrib.", agentContrib, 15, "%", "Agent Coord", agentContrib >= 15, false),
		mk(8, "Cost / Booking (avg)", costPerBooking, 5, "Jt", "Marketing", costPerBooking <= 5, true),
	}
}

/* ---------------- Panel 10: AI Alert & Action Plan ---------------- */

func buildAlerts(d *domain.Dashboard, h Headline) []domain.Alert {
	var out []domain.Alert
	add := func(sev, title, detail, pic, deadline, action string) {
		out = append(out, domain.Alert{Sev: sev, Title: title, Detail: detail, PIC: pic, Deadline: deadline, Action: action})
	}

	leadsToCV := pctOf(h.CV, h.ValidLeads)
	cvToPV := pctOf(h.PV, h.CV)
	pvToPurch := pctOf(h.Purchaser, h.PV)
	cancelRate := pctOf(h.Batal, h.Booking)

	if h.ValidLeads > 0 && leadsToCV < 20 {
		add("merah",
			fmt.Sprintf("Funnel Leads → CV rendah (%.1f%%, target ≥20%%)", leadsToCV),
			fmt.Sprintf("Dari %s valid leads hanya %d jadi Confirmed Visit.", grouped(int64(h.ValidLeads)), h.CV),
			"Marketing / SPV", "Hari ini",
			"Audit speed-to-lead & kualitas follow-up; aktifkan reminder WA otomatis.")
	}
	if h.CV > 0 && cvToPV < 70 {
		add("kuning",
			fmt.Sprintf("CV → PV rendah (%.1f%%, target ≥70%%)", cvToPV),
			fmt.Sprintf("%d Confirmed Visit, hanya %d jadi Project Visitor.", h.CV, h.PV),
			"Sales / Kadep", "H+3",
			"Follow-up jadwal visit & readiness produk; kunci ekspektasi.")
	}
	if h.PV > 0 && pvToPurch < 30 {
		add("kuning",
			fmt.Sprintf("PV → Purchaser rendah (%.1f%%, target ≥30%%)", pvToPurch),
			fmt.Sprintf("%d Project Visitor, %d jadi purchaser (source leads).", h.PV, h.Purchaser),
			"Sales / SPV", "Minggu ini",
			"Evaluasi closing, skema KPR, objection handling & campaign.")
	}
	// Project: booking > 0 tapi 0 akad.
	for _, p := range d.Projects {
		if p.Akad == 0 && (p.Proses+p.Batal) > 0 {
			add("merah",
				fmt.Sprintf("%s: 0 akad dari %d booking", p.Code, p.Total),
				fmt.Sprintf("%d proses, %d batal, ads %s tanpa konversi akad.", p.Proses, p.Batal, rpShortJt(p.Ads)),
				"Sales / Kadep", "H+3",
				"Stop / re-optimize campaign; review readiness produk.")
		}
	}
	if h.Booking > 0 && cancelRate > 15 {
		add("kuning",
			fmt.Sprintf("Cancel tinggi (%.1f%% dari booking)", cancelRate),
			fmt.Sprintf("%d batal dari %d booking. Risiko cash-in tertahan.", h.Batal, h.Booking),
			"KPR / Finance", "Minggu ini",
			"Eskalasi dokumen KPR & cek approval/DP/komitmen buyer.")
	}
	if h.Proses > 0 && h.Proses >= h.Akad {
		add("kuning",
			fmt.Sprintf("Pipeline besar belum akad (%d proses)", h.Proses),
			fmt.Sprintf("%d on proses vs %d akad — potensi tertahan di KPR.", h.Proses, h.Akad),
			"Sales / KPR", "Mingguan",
			"Dorong pipeline review mingguan & pendampingan akad.")
	}
	return out
}

// rpShortJt renders a Rupiah amount compactly in juta/miliar for alert text.
func rpShortJt(v int64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("Rp%.1f M", float64(v)/1e9)
	case v >= 1_000_000:
		return fmt.Sprintf("Rp%.1f Jt", float64(v)/1e6)
	default:
		return "Rp" + grouped(v)
	}
}
