package ingest

import (
	"sort"
	"time"

	"greenpark/sales/internal/domain"
)

// defaultTarget is the 2026 annual akad target (carried from the seed data; it
// is configuration, not derived from the upload).
const defaultTarget = 500

// monthLabels maps a month to its 3-letter Indonesian label for the trend.
var monthLabels = map[time.Month]string{
	time.January: "Jan", time.February: "Feb", time.March: "Mar",
	time.April: "Apr", time.May: "Mei", time.June: "Jun",
	time.July: "Jul", time.August: "Agu", time.September: "Sep",
	time.October: "Okt", time.November: "Nov", time.December: "Des",
}

// assemble folds the mapped funnel + sales (+ already-set stock/ads) into the
// full dashboard preview payload.
func assemble(res *Result, funnel []domain.FunnelStage, sd *salesData) {
	d := res.Preview
	d.Period = "Q1 + Q2 2026 · Jan–Jun"
	d.Updated = "" // stamped by the caller (no clock in the engine)

	booking := sd.booking // BRD: Tgl Booking tahun 2026 saja
	potential := estimatePotential(sd)

	d.Exec.Target2026 = defaultTarget
	d.Exec.Booking = booking
	d.Exec.Akad = sd.akad
	d.Exec.Proses = sd.proses
	d.Exec.Batal = sd.batal
	d.Exec.TotalPenjualan = sd.akad + sd.proses // penjualan aktif (non-batal)
	d.Exec.RevenueAkad = sd.cashIn
	d.Exec.PotentialRevenue = potential
	// Exec.AdsSpent* already set by mapAds.

	// Funnel: leads stages + a Purchaser stage. BRD: Purchaser = DATA PENJUALAN
	// dengan Sumber=LEADS & status bukan Batal/Cancel (NOT total booking).
	purchaser := sd.leadsPurchaser
	std30 := 30
	d.Funnel = append(funnel, domain.FunnelStage{
		Key: "Purchaser", Value: int64(purchaser), Target: int64(res.Headline.PV) * 30 / 100,
		Owner: "Sales", Std: &std30,
	})

	d.Projects = buildProjects(res, sd)
	d.Channels = buildChannels(sd)
	d.Sales = buildSales(sd)
	d.Agents = buildAgents(sd)
	d.Monthly = buildMonthly(sd)
	d.Events = buildEvents(sd)
	d.KPIs = buildKPIs(d, sd, res.Headline)
	d.Summary = buildSummary(d)
	d.Alerts = buildAlerts(d, res.Headline)
	d.ByProject = buildByProject(res, sd)
}

// estimatePotential approximates pipeline revenue: proses count × average akad
// price (falls back to 0 when there are no akad to price from).
func estimatePotential(sd *salesData) int64 {
	if sd.akad == 0 || sd.proses == 0 {
		return 0
	}
	avg := sd.cashIn / int64(sd.akad)
	return avg * int64(sd.proses)
}

func buildProjects(res *Result, sd *salesData) []domain.Project {
	out := make([]domain.Project, 0, len(sd.projOrder))
	for _, code := range sd.projOrder {
		a := sd.projects[code]
		name := CanonicalProjects[code]
		if name == "" {
			name = code
		}
		total := a.akad + a.proses + a.batal
		cpa := 0.0
		if a.akad > 0 {
			cpa = round1(float64(a.ads) / float64(a.akad) / 1_000_000)
		}
		out = append(out, domain.Project{
			Code: code, Name: name,
			Total: total, Akad: a.akad, Proses: a.proses, Batal: a.batal,
			Rev: a.rev, Ads: a.ads, CPA: cpa,
			Eff:   efficiency(a.akad, cpa),
			Cat:   "pendukung",
			Stock: res.stockByProj[code],
		})
	}
	return out
}

func efficiency(akad int, cpa float64) string {
	switch {
	case akad == 0:
		return "No Akad"
	case cpa > 0 && cpa <= 5:
		return "Excellent"
	case cpa > 0 && cpa <= 15:
		return "Good"
	default:
		return "Fair"
	}
}

func buildChannels(sd *salesData) []domain.Channel { return channelsFrom(sd.chanOrder, sd.channels) }

func channelsFrom(order []string, channels map[string]*chanAgg) []domain.Channel {
	out := make([]domain.Channel, 0, len(order))
	for _, name := range order {
		c := channels[name]
		conv := int(pctOf(c.akad, c.total))
		out = append(out, domain.Channel{Code: name, Name: name, Total: c.total, Akad: c.akad, Conv: conv})
	}
	return out
}

func buildSales(sd *salesData) []domain.SalesRep { return salesFrom(sd.repOrder, sd.reps) }

func salesFrom(order []string, reps map[string]*repAgg) []domain.SalesRep {
	out := make([]domain.SalesRep, 0, len(order))
	for _, name := range order {
		r := reps[name]
		total := r.akad + r.proses + r.batal
		out = append(out, domain.SalesRep{
			Name: name, Role: "Internal", Project: r.project,
			Akad: r.akad, Proses: r.proses, Batal: r.batal, Total: total, Conv: int(pctOf(r.akad, total)), Rev: r.rev,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Akad > out[j].Akad })
	return out
}

func buildMonthly(sd *salesData) []domain.MonthPoint { return monthlyFrom(sd.monthly) }

func monthlyFrom(monthly map[time.Month]*[2]int) []domain.MonthPoint {
	months := make([]time.Month, 0, len(monthly))
	for m := range monthly {
		months = append(months, m)
	}
	sort.Slice(months, func(i, j int) bool { return months[i] < months[j] })
	out := make([]domain.MonthPoint, 0, len(months))
	for _, m := range months {
		v := monthly[m]
		out = append(out, domain.MonthPoint{M: monthLabels[m], Akad: v[0], Booking: v[1]})
	}
	return out
}

// buildByProject builds the per-project view for every project that has leads
// or sales data, so the front-end project filter can drive all panels.
func buildByProject(res *Result, sd *salesData) map[string]domain.ProjectView {
	seen := map[string]bool{}
	var codes []string
	for _, c := range sd.projOrder {
		if !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	for c := range res.leadsByProj {
		if c != "—" && !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	out := make(map[string]domain.ProjectView, len(codes))
	for _, code := range codes {
		out[code] = projectView(sd.projects[code], res.leadsByProj[code])
	}
	return out
}

func projectView(pa *projAgg, pl *projLeads) domain.ProjectView {
	var v domain.ProjectView
	var leads, valid, cv, pv, purchaser int
	if pl != nil {
		leads, valid, cv, pv = pl.leads, pl.valid, pl.cv, pl.pv
	}
	if pa != nil {
		purchaser = pa.purchaser
		v.Exec = domain.Exec{
			Target2026: defaultTarget, Booking: pa.booking, Akad: pa.akad, Proses: pa.proses, Batal: pa.batal,
			TotalPenjualan: pa.akad + pa.proses, RevenueAkad: pa.rev, AdsSpent: pa.ads,
		}
		if pa.akad > 0 && pa.proses > 0 {
			v.Exec.PotentialRevenue = pa.rev / int64(pa.akad) * int64(pa.proses)
		}
		v.Channels = channelsFrom(pa.chanOrder, pa.channels)
		v.Sales = salesFrom(pa.repOrder, pa.reps)
		v.Agents = agentsFromMap(pa.agentOrder, pa.agents)
		v.Monthly = monthlyFrom(pa.monthly)
		v.Events = domain.Events{
			Attributed: domain.EventAttributed{Name: "Walk-in / Undangan", Booking: pa.eventBooking, Akad: pa.eventAkad, Conv: int(pctOf(pa.eventAkad, pa.eventBooking))},
			Note:       "Event/Walk-in untuk project ini (proxy DATA PENJUALAN).",
		}
	}
	std20, std70, std80, std30 := 20, 70, 80, 30
	v.Funnel = []domain.FunnelStage{
		{Key: "Leads", Value: int64(leads), Target: int64(leads), Owner: "Marketing", Std: nil},
		{Key: "Valid Leads", Value: int64(valid), Target: int64(leads) * 80 / 100, Owner: "Marketing", Std: &std80},
		{Key: "Confirmed Visit", Value: int64(cv), Target: int64(valid) * 20 / 100, Owner: "Sales", Std: &std20},
		{Key: "Project Visitor", Value: int64(pv), Target: int64(cv) * 70 / 100, Owner: "Sales", Std: &std70},
		{Key: "Purchaser", Value: int64(purchaser), Target: int64(pv) * 30 / 100, Owner: "Sales", Std: &std30},
	}
	if pl != nil {
		v.Reasons = reasonsFrom(pl.reasons)
	}
	return v
}

func buildSummary(d *domain.Dashboard) domain.Summary {
	e := d.Exec
	achievement, cancelRate, bookingToAkad := 0.0, 0.0, 0.0
	if e.Target2026 > 0 {
		achievement = round1(float64(e.Akad) / float64(e.Target2026) * 100)
	}
	if e.Booking > 0 {
		cancelRate = round1(float64(e.Batal) / float64(e.Booking) * 100)
		bookingToAkad = round1(float64(e.Akad) / float64(e.Booking) * 100)
	}
	avgCPA := 0.0
	if e.Akad > 0 {
		avgCPA = round1(float64(e.AdsSpent) / float64(e.Akad) / 1_000_000)
	}
	return domain.Summary{
		Target2026: e.Target2026, Akad: e.Akad, Booking: e.Booking, Proses: e.Proses, Batal: e.Batal,
		Achievement: achievement, GapToTarget: e.Target2026 - e.Akad, PipelineActive: e.Akad + e.Proses,
		CancelRate: cancelRate, BookingToAkad: bookingToAkad, CashIn: e.RevenueAkad,
		PotentialRevenue: e.PotentialRevenue, AdsSpent: e.AdsSpent, AvgCostPerAkad: avgCPA,
		TotalProjects: len(d.Projects), TotalSalesReps: len(d.Sales), StockSold: d.Stock.PctSold,
		Status: statusFor(bookingToAkad, achievement),
	}
}

func statusFor(bookingToAkad, achievement float64) string {
	switch {
	case achievement >= 60 && bookingToAkad >= 70:
		return "on-track"
	case achievement >= 25 || bookingToAkad >= 50:
		return "risk"
	default:
		return "off-track"
	}
}
