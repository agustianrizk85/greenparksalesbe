package repository

import (
	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/passwd"
)

// This file holds the representative seed data for the Sales dashboard. It
// mirrors the figures shown to the CEO war-room (DASHBOARD SALES_GREENPARK_2026,
// Q1+Q2 2026) and is intended to be replaced by a real data source (CRM / sales
// ledger) behind the same SalesRepository interface.
//
// Monetary values are in full Rupiah.

// intp is a small helper to take the address of an int literal (for nullable std).
func intp(v int) *int { return &v }

func seedExec() domain.Exec {
	return domain.Exec{
		Target2026:       500,
		Booking:          148,
		Akad:             104,
		Proses:           44,
		Batal:            31,
		TotalPenjualan:   117,
		RevenueAkad:      70822017000,
		PotentialRevenue: 29963161038,
		AdsSpent:         510999293,
		AdsSpentQ1:       301816718,
		AdsSpentQ2:       209182575,
	}
}

func seedMonthly() []domain.MonthPoint {
	return []domain.MonthPoint{
		{M: "Jan", Akad: 18, Booking: 27},
		{M: "Feb", Akad: 20, Booking: 29},
		{M: "Mar", Akad: 20, Booking: 30},
		{M: "Apr", Akad: 12, Booking: 24},
		{M: "Mei", Akad: 3, Booking: 25},
		{M: "Jun", Akad: 0, Booking: 12},
	}
}

// seedFunnel is the LEADS-only funnel per BRD BR-9: it ends at PURCHASER (78),
// the count of transactions whose source is LEADS and that did not cancel.
// Company sales come from many sources (walk-in, agent, referral, …), so Total
// Booking 148 / Akad 104 / Cash-In are NOT funnel stages — they are shown
// separately in the Executive snapshot (Panel 1) and Cash panel (Panel 8).
func seedFunnel() []domain.FunnelStage {
	return []domain.FunnelStage{
		{Key: "Leads", Value: 23157, Target: 23157, Owner: "Marketing", Std: nil},
		{Key: "Valid Leads", Value: 13381, Target: 18526, Owner: "Marketing", Std: intp(80)},
		{Key: "Confirmed Visit", Value: 740, Target: 2676, Owner: "Sales", Std: intp(20)},
		{Key: "Project Visitor", Value: 400, Target: 518, Owner: "Sales", Std: intp(70)},
		{Key: "Purchaser", Value: 78, Target: 120, Owner: "Sales", Std: intp(30)},
	}
}

func seedProjects() []domain.Project {
	return []domain.Project{
		{Code: "VERLIM3", Name: "Vertihauz Limo-3", Total: 24, Akad: 19, Proses: 3, Batal: 2, Rev: 14413330000, Ads: 60128211, CPA: 3.16, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 96, Closed: 70, Booking: 3, Avail: 23}},
		{Code: "ZHL", Name: "Z Hauz Limo", Total: 40, Akad: 22, Proses: 15, Batal: 3, Rev: 14399500000, Ads: 54340049, CPA: 2.47, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 65, Closed: 22, Booking: 10, Avail: 33}},
		{Code: "LHL", Name: "Le Hauz Limo", Total: 24, Akad: 15, Proses: 4, Batal: 5, Rev: 10609600000, Ads: 49114088, CPA: 3.27, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 39, Closed: 26, Booking: 4, Avail: 9}},
		{Code: "VERBUR", Name: "Vertihauz Cibubur", Total: 19, Akad: 10, Proses: 4, Batal: 5, Rev: 7628391000, Ads: 54717507, CPA: 5.47, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 75, Closed: 55, Booking: 0, Avail: 20}},
		{Code: "THC", Name: "The Hauz Cilodong", Total: 13, Akad: 7, Proses: 5, Batal: 1, Rev: 4926020000, Ads: 53475954, CPA: 7.64, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 23, Closed: 10, Booking: 3, Avail: 10}},
		{Code: "VERSAW", Name: "Vertihauz Sawangan", Total: 13, Akad: 8, Proses: 1, Batal: 4, Rev: 4009800000, Ads: 53193565, CPA: 6.65, Eff: "Excellent", Cat: "utama", Stock: domain.Stock{Total: 75, Closed: 51, Booking: 4, Avail: 34}},
		{Code: "VERUA", Name: "Vertihome Serua", Total: 13, Akad: 8, Proses: 2, Batal: 3, Rev: 4877600000, Ads: 53526938, CPA: 6.69, Eff: "Excellent", Cat: "pendukung", Stock: domain.Stock{Total: 26, Closed: 14, Booking: 3, Avail: 9}},
		{Code: "LHC", Name: "Le Hauz Cibubur", Total: 4, Akad: 4, Proses: 0, Batal: 0, Rev: 2517820000, Ads: 19696660, CPA: 4.92, Eff: "Excellent", Cat: "pendukung", Stock: domain.Stock{Total: 72, Closed: 65, Booking: 0, Avail: 7}},
		{Code: "MAVILL", Name: "Mahaba Village", Total: 1, Akad: 1, Proses: 0, Batal: 0, Rev: 600000000, Ads: 24616549, CPA: 24.62, Eff: "Good", Cat: "pendukung", Stock: domain.Stock{Total: 71, Closed: 69, Booking: 0, Avail: 2}},
		{Code: "THP", Name: "The Hauz Pancoran Mas", Total: 4, Akad: 1, Proses: 3, Batal: 0, Rev: 554256000, Ads: 34911866, CPA: 34.91, Eff: "Fair", Cat: "pembenahan", Stock: domain.Stock{Total: 18, Closed: 15, Booking: 3, Avail: 0}},
		{Code: "VERSER", Name: "Vertihome Serpong", Total: 4, Akad: 1, Proses: 1, Batal: 2, Rev: 629200000, Ads: 21763398, CPA: 21.76, Eff: "Good", Cat: "pembenahan", Stock: domain.Stock{Total: 28, Closed: 20, Booking: 0, Avail: 8}},
		{Code: "THPJ", Name: "The Hauz Premiere Jagakarsa", Total: 9, Akad: 0, Proses: 3, Batal: 6, Rev: 0, Ads: 31514508, CPA: 0, Eff: "No Akad", Cat: "pembenahan", Stock: domain.Stock{Total: 22, Closed: 0, Booking: 2, Avail: 20}},
	}
}

func seedChannels() []domain.Channel {
	return []domain.Channel{
		{Code: "WA", Name: "WhatsApp", Total: 51, Akad: 24, Conv: 47},
		{Code: "IG", Name: "Instagram (Meta Ads)", Total: 32, Akad: 15, Conv: 47},
		{Code: "WI", Name: "Walk-in / Undangan", Total: 32, Akad: 18, Conv: 56},
		{Code: "LD", Name: "Leads Tahun Lalu", Total: 15, Akad: 7, Conv: 47},
		{Code: "AG", Name: "Agent", Total: 11, Akad: 6, Conv: 55},
		{Code: "SF", Name: "Staff / Referal", Total: 6, Akad: 2, Conv: 33},
		{Code: "BG", Name: "Buyer Get Buyer", Total: 1, Akad: 1, Conv: 100},
	}
}

func seedSales() []domain.SalesRep {
	return []domain.SalesRep{
		{Name: "Ardan", Role: "Internal", Project: "LHL", Akad: 13, Proses: 6, Batal: 5, Total: 24, Conv: 54, Rev: 9291600000},
		{Name: "Ayu", Role: "Internal", Project: "VERLIM3", Akad: 11, Proses: 1, Batal: 1, Total: 13, Conv: 85, Rev: 8080030000},
		{Name: "Iwan", Role: "Internal", Project: "ZHL, VERBUR", Akad: 10, Proses: 9, Batal: 3, Total: 22, Conv: 45, Rev: 6774000000},
		{Name: "Martin", Role: "Internal", Project: "VERUA", Akad: 9, Proses: 2, Batal: 3, Total: 14, Conv: 64, Rev: 5603100000},
		{Name: "Muhammad Ilham", Role: "Internal", Project: "THC", Akad: 6, Proses: 4, Batal: 1, Total: 11, Conv: 55, Rev: 4219920000},
		{Name: "Yola", Role: "Internal", Project: "VERBUR", Akad: 6, Proses: 0, Batal: 2, Total: 8, Conv: 75, Rev: 4347291000},
		{Name: "Rizal", Role: "Internal", Project: "VERSAW, VERBUR", Akad: 6, Proses: 1, Batal: 4, Total: 11, Conv: 55, Rev: 3320350000},
		{Name: "Tria", Role: "Internal", Project: "VERLIM3, ZHL", Akad: 5, Proses: 5, Batal: 0, Total: 10, Conv: 50, Rev: 3162500000},
		{Name: "Seto", Role: "Internal", Project: "LHC", Akad: 4, Proses: 0, Batal: 0, Total: 4, Conv: 100, Rev: 2517820000},
		{Name: "Agent Usman", Role: "Agent", Project: "VERLIM3", Akad: 4, Proses: 2, Batal: 0, Total: 6, Conv: 67, Rev: 2317200000},
		{Name: "Doni", Role: "Internal", Project: "LHL, THP", Akad: 3, Proses: 4, Batal: 1, Total: 8, Conv: 38, Rev: 1872256000},
		{Name: "Erwin", Role: "Internal", Project: "THC, VERBUR", Akad: 2, Proses: 4, Batal: 2, Total: 8, Conv: 25, Rev: 1583200000},
		{Name: "Hasyim", Role: "Internal", Project: "VERBUR", Akad: 2, Proses: 1, Batal: 1, Total: 4, Conv: 50, Rev: 1627100000},
		{Name: "Teguh", Role: "Internal", Project: "ZHL, VERLIM3", Akad: 1, Proses: 1, Batal: 1, Total: 3, Conv: 33, Rev: 654800000},
		{Name: "OYI", Role: "Internal", Project: "VERLIM3", Akad: 1, Proses: 0, Batal: 0, Total: 1, Conv: 100, Rev: 760600000},
		{Name: "Asep", Role: "Internal", Project: "MAVILL", Akad: 1, Proses: 0, Batal: 0, Total: 1, Conv: 100, Rev: 600000000},
		{Name: "Rahadian", Role: "Internal", Project: "THPJ, VERSAW", Akad: 1, Proses: 0, Batal: 4, Total: 5, Conv: 20, Rev: 533200000},
		{Name: "Agent Rafli", Role: "Agent", Project: "VERLIM3, ZHL", Akad: 1, Proses: 2, Batal: 0, Total: 3, Conv: 33, Rev: 800000000},
		{Name: "Agent Imam", Role: "Agent", Project: "VERLIM3", Akad: 1, Proses: 0, Batal: 0, Total: 1, Conv: 100, Rev: 617900000},
		{Name: "Suseno", Role: "Internal", Project: "VERSER", Akad: 0, Proses: 1, Batal: 2, Total: 3, Conv: 0},
		{Name: "Agent Dedi", Role: "Agent", Project: "LHL", Akad: 0, Proses: 0, Batal: 1, Total: 1, Conv: 0},
	}
}

func seedReasonMeta() map[string]domain.ReasonMetaItem {
	return map[string]domain.ReasonMetaItem{
		"L1": {Stage: "Leads → CV", Target: "≥20% CV Rate"},
		"L2": {Stage: "CV → PV", Target: "≥70% PV Rate"},
		"L3": {Stage: "PV → Booking", Target: "≥30% Booking Rate"},
	}
}

func seedReasons() []domain.Reason {
	return []domain.Reason{
		{Code: "UNR", Name: "Unreachable", ID: "Tidak Terhubung", Layer: "L1", Count: 1376},
		{Code: "ENG", Name: "Not Engaged", ID: "Tidak Engage", Layer: "L1", Count: 1326},
		{Code: "NQ", Name: "Not Qualified", ID: "Data Tidak Cukup", Layer: "L1", Count: 420},
		{Code: "REJ", Name: "Rejected Visit", ID: "Menolak Visit", Layer: "L1", Count: 43},
		{Code: "SCH", Name: "Schedule Conflict", ID: "Jadwal Bentrok", Layer: "L2", Count: 115},
		{Code: "INF", Name: "Insufficient Info", ID: "Informasi Tidak Jelas", Layer: "L2", Count: 69},
		{Code: "COM", Name: "Weak Commitment", ID: "Komitmen Lemah", Layer: "L2", Count: 23},
		{Code: "FOR", Name: "Force Majeure", ID: "Kondisi Darurat", Layer: "L2", Count: 5},
		{Code: "EXP", Name: "Expectation Not Set", ID: "Ekspektasi Tak Terkunci", Layer: "L2", Count: 2},
		{Code: "REM", Name: "Reminder Failure", ID: "Reminder Gagal", Layer: "L2", Count: 1},
		{Code: "PRD", Name: "Product Mismatch", ID: "Produk Tidak Cocok", Layer: "L3", Count: 135},
		{Code: "FIN", Name: "Financially Infeasible", ID: "Tidak Lolos Finansial", Layer: "L3", Count: 65},
		{Code: "TIM", Name: "Timing Delay", ID: "Ditunda", Layer: "L3", Count: 47},
		{Code: "NST", Name: "No Next Step", ID: "Tidak Ada Next Step", Layer: "L3", Count: 31},
		{Code: "DM", Name: "Decision Maker Missing", ID: "Decision Maker Absen", Layer: "L3", Count: 31},
		{Code: "POL", Name: "Policy Constraint", ID: "Kendala Bank", Layer: "L3", Count: 9},
		{Code: "CMP", Name: "Competitor Won", ID: "Kalah Kompetitor", Layer: "L3", Count: 7},
	}
}

func seedAgents() []domain.Agent {
	return []domain.Agent{
		{Name: "Agent Usman", Project: "VERLIM3", Leads: "—", Akad: 4, Total: 6, Conv: 67, Status: "Active Productive"},
		{Name: "Agent Rafli", Project: "VERLIM3", Leads: "—", Akad: 1, Total: 3, Conv: 33, Status: "Active Low Conv"},
		{Name: "Agent Imam", Project: "VERLIM3", Leads: "—", Akad: 1, Total: 1, Conv: 100, Status: "Active Productive"},
		{Name: "Agent Dedi", Project: "LHL", Leads: "—", Akad: 0, Total: 1, Conv: 0, Status: "Need Reactivation"},
	}
}

func seedStock() domain.MasterStock {
	return domain.MasterStock{Total: 610, Closed: 417, Booking: 32, Hold: 0, Avail: 175, PctSold: 68.4}
}

func seedEvents() domain.Events {
	return domain.Events{
		Attributed: domain.EventAttributed{Name: "Walk-in / Undangan (Event & Open House)", Booking: 32, Akad: 18, Conv: 56},
		Note:       "Metrik biaya/leads event per acara menunggu input Event Form & QR Form.",
	}
}

func seedAlerts() []domain.Alert {
	return []domain.Alert{
		{Sev: "merah", Title: "Funnel Leads → CV hanya 5,4% (target ≥20%)", Detail: "Dari 13.255 valid leads hanya 711 jadi Confirmed Visit. 1.376 leads Unreachable + 1.326 Not Engaged.", PIC: "Marketing / SPV", Deadline: "Hari ini", Action: "Audit speed-to-lead & kualitas follow-up; aktifkan reminder WA otomatis."},
		{Sev: "merah", Title: "THPJ: 0 akad dari 8 booking", Detail: "6 booking batal. Spend iklan Rp31,5 Jt tanpa konversi.", PIC: "Sales / Kadep", Deadline: "H+3", Action: "Stop / re-optimize campaign; review readiness produk."},
		{Sev: "merah", Title: "Sales perlu coaching", Detail: "Suseno 0 akad / 3 prospek; Rahadian conv 20% (4 batal).", PIC: "SPV Sales", Deadline: "Mingguan", Action: "Bedah script follow-up & closing; pendampingan SPV."},
		{Sev: "kuning", Title: "Data integrity akad belum konsisten", Detail: "Headline 104 vs per-project 96 vs sales breakdown 87 akibat ketidakseragaman kode project (THP J↔THPJ).", PIC: "AI Dept", Deadline: "Minggu ini", Action: "Samakan kode project & tetapkan satu sumber data tunggal."},
		{Sev: "kuning", Title: "Batal 31 unit (20,9% dari booking)", Detail: "44 booking masih proses, 31 batal. Risiko cash-in tertahan ±Rp29,96 M.", PIC: "KPR / Finance", Deadline: "Minggu ini", Action: "Eskalasi dokumen KPR & pendampingan akad pipeline aktif."},
		{Sev: "kuning", Title: "Cost/Akad MAVILL & THP sangat tinggi", Detail: "MAVILL Rp24,6 Jt · THP Rp34,9 Jt per akad — jauh di atas baseline Rp2–8 Jt.", PIC: "Marketing", Deadline: "H+3", Action: "Realokasi budget ke project Excellent (VERLIM3, ZHL, LHL)."},
		{Sev: "hijau", Title: "Top performer: Ardan (13) & Ayu (11)", Detail: "23% akad disumbang 2 sales. Knowledge transfer ke seluruh tim.", PIC: "SPV Sales", Deadline: "Mingguan", Action: "Replikasi playbook closing mereka & jaga momentum mesin utama."},
	}
}

func seedKPIs() []domain.KPI {
	return []domain.KPI{
		{No: 1, Name: "Valid Leads Rate", Value: 57.8, Target: 80, Unit: "%", Owner: "Marketing", Good: false},
		{No: 2, Name: "Leads → CV Rate", Value: 5.5, Target: 20, Unit: "%", Owner: "Sales", Good: false},
		{No: 3, Name: "CV → PV Rate", Value: 54.1, Target: 70, Unit: "%", Owner: "Sales", Good: false},
		{No: 4, Name: "PV → Purchaser Rate", Value: 19.5, Target: 30, Unit: "%", Owner: "Sales", Good: false},
		{No: 5, Name: "Booking → Akad Rate", Value: 70.3, Target: 70, Unit: "%", Owner: "Sales/KPR", Good: true},
		{No: 6, Name: "Cash-In Achievement", Value: 70.3, Target: 100, Unit: "%", Owner: "Finance/KPR", Good: false},
		{No: 7, Name: "Agent Booking Contrib.", Value: 7.4, Target: 15, Unit: "%", Owner: "Agent Coord", Good: false},
		{No: 8, Name: "Cost / Booking (avg)", Value: 3.45, Target: 5, Unit: "Jt", Owner: "Marketing", Good: true, LowerBetter: true},
	}
}

// seedUsers creates the default accounts. Change these immediately in any real
// deployment. Default credentials: admin/admin123 and viewer/viewer123.
func seedUsers() []storeUser {
	mk := func(id, username, name string, role domain.Role, password string) storeUser {
		salt := passwd.NewSalt()
		return storeUser{
			ID:           id,
			Username:     username,
			Name:         name,
			Role:         role,
			Salt:         salt,
			PasswordHash: passwd.Hash(password, salt),
		}
	}
	return []storeUser{
		mk("usr-admin", "admin", "Administrator", domain.RoleAdmin, "admin123"),
		mk("usr-viewer", "viewer", "Viewer", domain.RoleViewer, "viewer123"),
	}
}
