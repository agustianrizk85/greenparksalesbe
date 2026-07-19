package http

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"greenpark/sales/internal/domain"
)

/* ---------------------------- konsumen screening ---------------------------- */

// screeningQuestions returns the questionnaire. Any authenticated sales user can
// read it; staff render the Active ones, the Kadep sees the full set to manage.
func (h *Handler) screeningQuestions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.ScreeningQuestions())
}

// setScreeningQuestions replaces the whole questionnaire (admin/Kadep only).
func (h *Handler) setScreeningQuestions(w http.ResponseWriter, r *http.Request) {
	qs, ok := decode[[]domain.ScreeningQuestion](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetScreeningQuestions(qs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.svc.ScreeningQuestions())
}

// screeningSubmissions lists completed screenings. The Kadep (admin) sees all;
// a staff member sees only their own, so the qualification pipeline stays private
// per salesperson while the head reviews everything.
func (h *Handler) screeningSubmissions(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(userCtxKey).(domain.User)
	all := h.svc.ScreeningSubmissions()
	if u.Role == domain.RoleAdmin {
		writeJSON(w, http.StatusOK, all)
		return
	}
	mine := make([]domain.ScreeningSubmission, 0, len(all))
	for _, s := range all {
		if strings.EqualFold(s.By, u.Username) {
			mine = append(mine, s)
		}
	}
	writeJSON(w, http.StatusOK, mine)
}

type assessReq struct {
	Consumer string                   `json:"consumer"`
	Phone    string                   `json:"phone"`
	Project  string                   `json:"project"`
	Unit     string                   `json:"unit"`
	Price    int64                    `json:"price"`
	Answers  []domain.ScreeningAnswer `json:"answers"`
	// Result is the AI verdict the frontend obtained from the CENTRAL AI gateway
	// (auth service → Ollama). Optional: when absent/invalid the backend computes
	// the deterministic rule-based score instead, so a verdict is always returned
	// even when AI is off. AI is unified through auth — this service holds no key.
	Result *domain.ScreeningResult `json:"result"`
}

// assessScreening persists a completed screening and returns the verdict. The AI
// scoring runs on the frontend via the shared auth AI gateway (Ollama) — the same
// central key every division uses — so this service never calls an LLM itself.
// When the frontend supplies no (valid) AI result, we fall back to the built-in
// rule-based scorer. Either way the completed screening is saved (owned by the
// requesting staff).
func (h *Handler) assessScreening(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[assessReq](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Consumer) == "" {
		writeError(w, http.StatusBadRequest, "nama konsumen wajib diisi")
		return
	}
	u, _ := r.Context().Value(userCtxKey).(domain.User)

	sub := domain.ScreeningSubmission{
		Consumer:  strings.TrimSpace(req.Consumer),
		Phone:     strings.TrimSpace(req.Phone),
		Project:   strings.TrimSpace(req.Project),
		Unit:      strings.TrimSpace(req.Unit),
		Price:     req.Price,
		Answers:   req.Answers,
		By:        u.Username,
		ByName:    u.Name,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// Use the AI verdict from the auth gateway when the frontend supplied a valid
	// one; otherwise score with the deterministic rules.
	used := false
	if req.Result != nil {
		if v := canonVerdict(req.Result.Verdict); v != "" {
			res := *req.Result
			res.Verdict = v
			res.Score = clampInt(res.Score, 0, 100)
			res.Source = "ai"
			res.Strengths, res.Risks, res.Recommendations = trimList(res.Strengths), trimList(res.Risks), trimList(res.Recommendations)
			sub.Result = res
			used = true
		}
	}
	if !used {
		sub.Result = ruleBasedScreening(h.svc.ScreeningQuestions(), sub)
	}

	saved, err := h.svc.SaveScreeningSubmission(sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// canonVerdict maps loose verdict text onto the canonical codes; "" = invalid.
func canonVerdict(v string) string {
	s := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(v), " ", "_"))
	switch {
	case s == "":
		return ""
	case strings.Contains(s, "tidak"):
		return "tidak_layak"
	case strings.Contains(s, "syarat") || strings.Contains(s, "kondisi"):
		return "bersyarat"
	case strings.Contains(s, "layak"):
		return "layak"
	case strings.Contains(s, "review") || strings.Contains(s, "tinjau"):
		return "review"
	default:
		return ""
	}
}

func trimList(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if t := strings.TrimSpace(x); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// deleteScreeningSubmission removes a completed screening (admin/Kadep only).
func (h *Handler) deleteScreeningSubmission(w http.ResponseWriter, r *http.Request) {
	ok, err := h.svc.DeleteScreeningSubmission(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

/* ---------------------------- rule-based fallback ---------------------------- */

// ruleBasedScreening is the deterministic scorer used when AI is unavailable. It
// keys off the seed question ids (with a label-keyword fallback for customised
// questionnaires) and applies the common Indonesian KPR checks: DSR <= 35%, a
// clean SLIK (hard gate), DP adequacy and document completeness.
func ruleBasedScreening(questions []domain.ScreeningQuestion, sub domain.ScreeningSubmission) domain.ScreeningResult {
	byID := make(map[string]string, len(sub.Answers))
	for _, a := range sub.Answers {
		byID[a.QuestionID] = strings.TrimSpace(a.Value)
	}
	find := func(id string, keywords ...string) string {
		if v := byID[id]; v != "" {
			return v
		}
		for _, a := range sub.Answers {
			l := strings.ToLower(a.Label)
			for _, k := range keywords {
				if strings.Contains(l, k) && strings.TrimSpace(a.Value) != "" {
					return strings.TrimSpace(a.Value)
				}
			}
		}
		return ""
	}

	income := parseAmount(find("scq-income", "penghasilan", "income", "gaji", "pendapatan"))
	otherInst := parseAmount(find("scq-otherinstallment", "cicilan lain", "cicilan/utang", "utang", "hutang"))
	dp := parseAmount(find("scq-dp", "uang muka", "down payment"))
	slikRaw := find("scq-slik", "slik", "bi checking", "riwayat kredit")
	docsRaw := find("scq-docs", "dokumen")
	price := float64(sub.Price)

	principal := math.Max(0, price-dp)
	installment := annuityMonthly(principal, 0.10, 15)
	disposable := income - otherInst

	var strengths, risks, recs []string
	score := 55
	dsr := 0.0
	hasDSR := income > 0 && price > 0

	if hasDSR {
		if disposable <= 0 {
			dsr = math.Inf(1)
			score -= 30
			risks = append(risks, "Cicilan lain sudah menghabiskan penghasilan — tidak ada kapasitas untuk angsuran baru")
		} else {
			dsr = installment / disposable
			switch {
			case dsr <= 0.35:
				score += 22
				strengths = append(strengths, fmt.Sprintf("Estimasi DSR sehat (~%.0f%%): angsuran ±%s/bln vs kemampuan bayar %s/bln", dsr*100, rpCompact(installment), rpCompact(disposable)))
			case dsr <= 0.45:
				score += 4
				risks = append(risks, fmt.Sprintf("Estimasi DSR agak tinggi (~%.0f%%): angsuran ±%s/bln", dsr*100, rpCompact(installment)))
				recs = append(recs, "Pertimbangkan tenor lebih panjang atau tambah DP agar angsuran turun")
			default:
				score -= 25
				risks = append(risks, fmt.Sprintf("Estimasi DSR terlalu tinggi (~%.0f%%): angsuran ±%s/bln melebihi kapasitas", dsr*100, rpCompact(installment)))
				recs = append(recs, "Tawarkan unit lebih terjangkau, tenor lebih panjang, atau tambah DP")
			}
		}
	} else {
		risks = append(risks, "Data penghasilan/harga rumah belum lengkap — DSR belum bisa dihitung")
	}

	if price > 0 && dp > 0 {
		if dp >= 0.2*price {
			score += 10
			strengths = append(strengths, fmt.Sprintf("DP memadai (%s ≈ %.0f%% dari harga)", rpCompact(dp), dp/price*100))
		} else {
			score -= 5
			risks = append(risks, fmt.Sprintf("DP masih di bawah 20%% (%s ≈ %.0f%%)", rpCompact(dp), dp/price*100))
			recs = append(recs, "Bantu konsumen menyiapkan DP minimal 20% atau cari skema DP ringan")
		}
	}

	slikBad := false
	switch {
	case slikRaw == "":
		risks = append(risks, "Status SLIK/BI Checking belum diketahui — perlu dicek")
	case parseBool(slikRaw):
		score += 15
		strengths = append(strengths, "Riwayat kredit (SLIK) bersih")
	default:
		slikBad = true
		score -= 40
		risks = append(risks, "Riwayat kredit (SLIK/BI Checking) bermasalah")
		recs = append(recs, "Sarankan konsumen membereskan tunggakan/kolektibilitas sebelum pengajuan KPR")
	}

	if docsRaw != "" {
		if parseBool(docsRaw) {
			score += 5
			strengths = append(strengths, "Dokumen pengajuan sudah lengkap")
		} else {
			recs = append(recs, "Lengkapi dokumen KPR (KTP, KK, slip gaji, rekening koran, NPWP)")
		}
	}

	// Not enough to judge the two most important factors → be honest.
	if !hasDSR && slikRaw == "" {
		return domain.ScreeningResult{
			Verdict: "review", Score: 0, Source: "rules",
			Summary:         "Data kunci (penghasilan & riwayat kredit) belum cukup untuk penilaian otomatis.",
			Risks:           []string{"Penghasilan/harga rumah belum lengkap", "Status SLIK belum diketahui"},
			Recommendations: []string{"Lengkapi data screening lalu nilai ulang, atau aktifkan penilaian AI"},
		}
	}

	score = clampInt(score, 0, 100)
	verdict := "bersyarat"
	switch {
	case slikBad || score < 45:
		verdict = "tidak_layak"
	case score >= 70 && hasDSR:
		verdict = "layak"
	}

	label := map[string]string{"layak": "Layak", "bersyarat": "Layak dengan syarat", "tidak_layak": "Belum layak"}[verdict]
	summary := fmt.Sprintf("%s (skor %d/100) berdasarkan penilaian aturan cepat.", label, score)
	if hasDSR && !math.IsInf(dsr, 1) {
		summary += fmt.Sprintf(" Estimasi angsuran KPR ±%s/bln (DSR ~%.0f%%).", rpCompact(installment), dsr*100)
	}

	return domain.ScreeningResult{
		Verdict: verdict, Score: score, Source: "rules",
		Summary: summary, Strengths: strengths, Risks: risks, Recommendations: recs,
	}
}

// annuityMonthly returns the level monthly payment for a loan (annualRate as a
// fraction, e.g. 0.10), or 0 when the inputs are non-positive.
func annuityMonthly(principal, annualRate float64, years int) float64 {
	n := float64(years * 12)
	if principal <= 0 || n <= 0 {
		return 0
	}
	i := annualRate / 12
	if i == 0 {
		return principal / n
	}
	return principal * i / (1 - math.Pow(1+i, -n))
}

// parseAmount extracts the digits from a currency/number string ("Rp 8.000.000"
// → 8000000). Returns 0 when there are no digits.
func parseAmount(s string) float64 {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(b.String(), 64)
	return v
}

// parseBool reads an affirmative answer from loose Indonesian/English input.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "ya", "yes", "y", "1", "sudah", "bersih", "lengkap", "ada":
		return true
	}
	return false
}

func rpCompact(n float64) string {
	var s string
	switch {
	case n >= 1e9:
		s = fmt.Sprintf("Rp%.1f M", n/1e9)
	case n >= 1e6:
		s = fmt.Sprintf("Rp%.1f jt", n/1e6)
	case n >= 1e3:
		s = fmt.Sprintf("Rp%.0f rb", n/1e3)
	default:
		s = fmt.Sprintf("Rp%.0f", n)
	}
	return strings.Replace(s, ".", ",", 1)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
