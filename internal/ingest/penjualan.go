package ingest

import (
	"strings"
	"time"
)

// projAgg accumulates one project's sales position, including the per-project
// breakdowns the project filter needs (channels, sales, agents, trend).
type projAgg struct {
	akad, proses, batal int
	booking, purchaser  int
	eventBooking        int
	eventAkad           int
	rev                 int64
	ads                 int64
	channels            map[string]*chanAgg
	chanOrder           []string
	reps                map[string]*repAgg
	repOrder            []string
	agents              map[string]*repAgg
	agentOrder          []string
	monthly             map[time.Month]*[2]int
}

func newProjAgg() *projAgg {
	return &projAgg{
		channels: map[string]*chanAgg{},
		reps:     map[string]*repAgg{},
		agents:   map[string]*repAgg{},
		monthly:  map[time.Month]*[2]int{},
	}
}

func (p *projAgg) chan_(name string) *chanAgg {
	if a, ok := p.channels[name]; ok {
		return a
	}
	a := &chanAgg{}
	p.channels[name] = a
	p.chanOrder = append(p.chanOrder, name)
	return a
}
func (p *projAgg) rep(name string) *repAgg {
	if a, ok := p.reps[name]; ok {
		return a
	}
	a := &repAgg{}
	p.reps[name] = a
	p.repOrder = append(p.repOrder, name)
	return a
}
func (p *projAgg) agent(name string) *repAgg {
	if a, ok := p.agents[name]; ok {
		return a
	}
	a := &repAgg{}
	p.agents[name] = a
	p.agentOrder = append(p.agentOrder, name)
	return a
}
func (p *projAgg) month(m time.Month) *[2]int {
	if a, ok := p.monthly[m]; ok {
		return a
	}
	a := &[2]int{}
	p.monthly[m] = a
	return a
}

// repAgg accumulates one deal-closer's performance.
type repAgg struct {
	akad, proses, batal int
	rev                 int64
	project             string
}

// chanAgg accumulates one source/channel's contribution.
type chanAgg struct {
	total, akad int
}

// salesData is the mapped DATA PENJUALAN, shared with the ads + assemble steps.
type salesData struct {
	akad, proses, batal int
	booking             int // BRD: transaksi dengan Tgl Booking tahun 2026 saja
	cashIn              int64
	leadsPurchaser      int // BRD: Sumber=LEADS & status != Batal
	projects            map[string]*projAgg
	reps                map[string]*repAgg
	channels            map[string]*chanAgg
	agents              map[string]*repAgg // Sumber=AGENT, keyed by deal closer
	agentOrder          []string
	eventBooking        int // Sumber=Walk-in/Undangan/Visitor (WI): all transactions
	eventAkad           int
	monthly             map[time.Month]*[2]int // month → {akad, booking}
	projOrder           []string               // first-seen order for stable output
	repOrder            []string
	chanOrder           []string
}

func isAgentSource(src string) bool { return hasAny(src, "agent") }
func isEventSource(src string) bool { return hasAny(src, "visitor", "walk", "undangan", "event") }
func isLeadsSource(src string) bool { return strings.EqualFold(trim(src), "LEADS") }

// channelCategory maps a (cleaned) Platform value to one of the BRD standard
// source categories. Robust to spelling variants if the raw column is used.
func channelCategory(raw string) string {
	s := collapseSpace(raw)
	if s == "" {
		return "Lainnya"
	}
	ls := toLower(s)
	switch {
	case contains(ls, "instagram"), contains(ls, "meta"):
		return "Instagram (Meta Ads)"
	case contains(ls, "whatsapp"), ls == "wa":
		return "WhatsApp"
	case contains(ls, "walk"), contains(ls, "undangan"), ls == "wi":
		return "Walk-in / Undangan"
	case contains(ls, "agent"):
		return "Agent"
	case contains(ls, "staff"), contains(ls, "referal"), contains(ls, "referral"):
		return "Staff / Referal"
	case contains(ls, "buyer get buyer"), contains(ls, "bgb"):
		return "Buyer Get Buyer"
	case contains(ls, "leads"):
		return "Leads - Lainnya"
	default:
		return s
	}
}

// knownChannels are the recognised source labels in the Sumber column.
var knownChannels = map[string]bool{
	"LEADS": true, "AGENT": true, "VISITOR (WI)": true,
	"STAFF / REFERAL": true, "BUYER GET BUYER (BGB)": true,
}

func classifyStatus(s string) string {
	switch {
	case hasAny(s, "akad"):
		return "akad"
	case hasAny(s, "proses"):
		return "proses"
	case hasAny(s, "batal", "cancel", "gugur"):
		return "batal"
	default:
		return ""
	}
}

func mapPenjualan(rs rows, res *Result) *salesData {
	cProj := rs.col("project")
	cName := rs.col("nama", "name")
	cPhone := rs.col("no hp", "phone")
	cCloser := rs.col("deal closer", "sales")
	cBook := rs.col("tgl booking", "booking")
	cAkadDate := rs.col("tgl akad")
	cRev := rs.col("revenue")
	cStatus := rs.col("status")
	cSumber := rs.col("sumber")
	// Channel panel uses the CLEANED Platform column (the last "Platform" header,
	// which carries the standard categories), not the raw Sumber.
	cChannel := -1
	if pc := rs.colAll("platform"); len(pc) > 0 {
		cChannel = pc[len(pc)-1]
	}

	sd := &salesData{
		projects: map[string]*projAgg{},
		reps:     map[string]*repAgg{},
		channels: map[string]*chanAgg{},
		agents:   map[string]*repAgg{},
		monthly:  map[time.Month]*[2]int{},
	}

	for _, miss := range []struct {
		idx  int
		name string
	}{{cProj, "Project"}, {cName, "Nama"}, {cStatus, "Status"}} {
		if miss.idx < 0 {
			res.addIssue("Kolom Wajib", SevError, sheetPenjualan, 0, "kolom '"+miss.name+"' tidak ditemukan")
		}
	}
	if cProj < 0 || cName < 0 || cStatus < 0 {
		return sd
	}

	seenPhone := map[string]int{}

	for i := range rs.data {
		name := trim(rs.cell(i, cName))
		if name == "" {
			continue
		}
		row := i + 2 // 1-based Excel row (header on row 1)

		// --- status ---
		rawStatus := trim(rs.cell(i, cStatus))
		status := classifyStatus(rawStatus)
		if status == "" {
			res.addIssue("Status Transaksi", SevWarning, sheetPenjualan, row,
				"status tidak dikenal: "+orDash(rawStatus))
			continue
		}

		// BRD: Akad 2026 dihitung dari Tgl Akad tahun 2026. Baris akad dengan
		// tanggal akad di luar 2026 tidak masuk hitungan akad/cash-in.
		if status == "akad" && cAkadDate >= 0 {
			if t, ok := ParseDate(rs.cell(i, cAkadDate)); ok && t.Year() != 2026 {
				continue
			}
		}

		// --- project ---
		code, known := NormalizeProject(rs.cell(i, cProj))
		if !known {
			res.addIssue("Nama Project", SevWarning, sheetPenjualan, row,
				"project tidak dikenal: "+orDash(rs.cell(i, cProj)))
		}
		pa := sd.proj(code)

		// --- revenue (required & well-formed for akad) ---
		var rev int64
		if cRev >= 0 {
			if v, ok := ParseRupiah(rs.cell(i, cRev)); ok {
				rev = v
			} else if status == "akad" {
				res.addIssue("Format Revenue", SevWarning, sheetPenjualan, row,
					"revenue akad kosong/tidak terbaca: "+orDash(rs.cell(i, cRev)))
			}
		}

		// --- booking date format ---
		if cBook >= 0 {
			if raw := trim(rs.cell(i, cBook)); raw != "" {
				if _, ok := ParseDate(raw); !ok {
					res.addIssue("Format Tanggal", SevWarning, sheetPenjualan, row,
						"tgl booking tidak terbaca: "+raw)
				}
			}
		}

		// --- duplicate phone (info) ---
		if cPhone >= 0 {
			if ph := NormalizePhone(rs.cell(i, cPhone)); ph != "" {
				if prev, dup := seenPhone[ph]; dup {
					res.addIssue("Duplikat No HP", SevInfo, sheetPenjualan, row,
						"no HP sama dengan baris "+itoa(prev))
				} else {
					seenPhone[ph] = row
				}
			}
		}

		// --- channel category (BRD 7 standar) dari kolom Platform yang sudah bersih ---
		if cChannel >= 0 {
			cat := channelCategory(rs.cell(i, cChannel))
			ch := sd.chan_(cat)
			ch.total++
			pch := pa.chan_(cat)
			pch.total++
			if status == "akad" {
				ch.akad++
				pch.akad++
			}
		}

		// --- source (Sumber) → purchaser / agent / event + validasi ---
		if cSumber >= 0 {
			src := collapseSpace(rs.cell(i, cSumber))
			if src != "" {
				if !knownChannels[src] {
					res.addIssue("Source/Channel", SevWarning, sheetPenjualan, row,
						"source tidak dikenal: "+src)
				}
				// BRD: Purchaser = Sumber LEADS & status bukan Batal/Cancel.
				if isLeadsSource(src) && status != "batal" {
					sd.leadsPurchaser++
					pa.purchaser++
				}
				// Panel 9: Agent contribution (Sumber=Agent), by deal closer.
				if isAgentSource(src) {
					name := trim(rs.cell(i, cCloser))
					if name == "" {
						name = "Agent (tanpa nama)"
					}
					ag := sd.agent(name)
					pag := pa.agent(name)
					if ag.project == "" {
						ag.project = code
					}
					if pag.project == "" {
						pag.project = code
					}
					switch status {
					case "akad":
						ag.akad++
						ag.rev += rev
						pag.akad++
						pag.rev += rev
					case "proses":
						ag.proses++
						pag.proses++
					case "batal":
						ag.batal++
						pag.batal++
					}
				}
				// Panel 9: Event / Walk-in / Undangan proxy.
				if isEventSource(src) {
					sd.eventBooking++
					pa.eventBooking++
					if status == "akad" {
						sd.eventAkad++
						pa.eventAkad++
					}
				}
			}
		}

		// --- sales rep (validasi nama sales) ---
		closer := trim(rs.cell(i, cCloser))
		if closer == "" {
			res.addIssue("Nama Sales", SevWarning, sheetPenjualan, row, "deal closer kosong")
		} else {
			ra := sd.rep(closer)
			pra := pa.rep(closer)
			if ra.project == "" {
				ra.project = code
			}
			if pra.project == "" {
				pra.project = code
			}
			switch status {
			case "akad":
				ra.akad++
				ra.rev += rev
				pra.akad++
				pra.rev += rev
			case "proses":
				ra.proses++
				pra.proses++
			case "batal":
				ra.batal++
				pra.batal++
			}
		}

		// --- aggregate ---
		switch status {
		case "akad":
			sd.akad++
			sd.cashIn += rev
			pa.akad++
			pa.rev += rev
		case "proses":
			sd.proses++
			pa.proses++
		case "batal":
			sd.batal++
			pa.batal++
		}

		// BRD: Booking 2026 = transaksi dengan Tgl Booking tahun 2026 saja.
		// Booking 2025 yang akad 2026 TIDAK dihitung sebagai booking (tapi tetap
		// terhitung sebagai akad). Tanggal kosong/tak terbaca → dianggap 2026.
		booking2026 := true
		if cBook >= 0 {
			if t, ok := ParseDate(rs.cell(i, cBook)); ok && t.Year() != 2026 {
				booking2026 = false
			}
		}
		if booking2026 {
			sd.booking++
			pa.booking++
		}

		// --- monthly trend (akad by Tgl Akad, booking by Tgl Booking) ---
		// Restricted to the reporting period so prior-year carry-over dates do
		// not bleed into the trend.
		if status == "akad" && cAkadDate >= 0 {
			if t, ok := ParseDate(rs.cell(i, cAkadDate)); ok && inPeriod(t) {
				sd.month(t.Month())[0]++
				pa.month(t.Month())[0]++
			}
		}
		if status != "batal" && cBook >= 0 { // booking = akad + proses
			if t, ok := ParseDate(rs.cell(i, cBook)); ok && inPeriod(t) {
				sd.month(t.Month())[1]++
				pa.month(t.Month())[1]++
			}
		}
	}

	h := &res.Headline
	h.Akad, h.Proses, h.Batal = sd.akad, sd.proses, sd.batal
	h.Booking = sd.booking          // BRD: Tgl Booking tahun 2026 saja (termasuk Batal 2026)
	h.Purchaser = sd.leadsPurchaser // BRD: Sumber=LEADS & bukan Batal
	h.CashIn = sd.cashIn
	return sd
}

// proj / rep / chan_ fetch-or-create an aggregate, preserving first-seen order.
func (sd *salesData) proj(code string) *projAgg {
	if a, ok := sd.projects[code]; ok {
		return a
	}
	a := newProjAgg()
	sd.projects[code] = a
	sd.projOrder = append(sd.projOrder, code)
	return a
}

func (sd *salesData) rep(name string) *repAgg {
	if a, ok := sd.reps[name]; ok {
		return a
	}
	a := &repAgg{}
	sd.reps[name] = a
	sd.repOrder = append(sd.repOrder, name)
	return a
}

func (sd *salesData) agent(name string) *repAgg {
	if a, ok := sd.agents[name]; ok {
		return a
	}
	a := &repAgg{}
	sd.agents[name] = a
	sd.agentOrder = append(sd.agentOrder, name)
	return a
}

func (sd *salesData) month(m time.Month) *[2]int {
	if a, ok := sd.monthly[m]; ok {
		return a
	}
	a := &[2]int{}
	sd.monthly[m] = a
	return a
}

func (sd *salesData) chan_(name string) *chanAgg {
	if a, ok := sd.channels[name]; ok {
		return a
	}
	a := &chanAgg{}
	sd.channels[name] = a
	sd.chanOrder = append(sd.chanOrder, name)
	return a
}
