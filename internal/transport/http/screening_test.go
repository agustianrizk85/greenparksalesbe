package http

import (
	"math"
	"testing"

	"greenpark/sales/internal/domain"
)

func TestAnnuityMonthly(t *testing.T) {
	// 300jt, 10%/yr, 15yr → ~Rp3.22jt/bln (standard annuity).
	got := annuityMonthly(300_000_000, 0.10, 15)
	if math.Abs(got-3_224_000) > 5_000 {
		t.Fatalf("annuityMonthly = %.0f, want ~3.224jt", got)
	}
	if annuityMonthly(0, 0.10, 15) != 0 {
		t.Fatalf("zero principal must give 0")
	}
	// Zero interest → straight division.
	if got := annuityMonthly(120_000_000, 0, 10); math.Abs(got-1_000_000) > 1 {
		t.Fatalf("zero-rate annuity = %.0f, want 1jt", got)
	}
}

func mkAnswer(id, label, val string) domain.ScreeningAnswer {
	return domain.ScreeningAnswer{QuestionID: id, Label: label, Value: val}
}

func TestRuleBasedScreening_Layak(t *testing.T) {
	sub := domain.ScreeningSubmission{
		Price: 300_000_000,
		Answers: []domain.ScreeningAnswer{
			mkAnswer("scq-income", "Penghasilan bersih per bulan", "20000000"),
			mkAnswer("scq-otherinstallment", "Total cicilan lain", "0"),
			mkAnswer("scq-dp", "Kesiapan uang muka", "80000000"),
			mkAnswer("scq-slik", "SLIK bersih?", "ya"),
			mkAnswer("scq-docs", "Dokumen lengkap?", "ya"),
		},
	}
	res := ruleBasedScreening(nil, sub)
	if res.Verdict != "layak" {
		t.Fatalf("verdict = %q (score %d), want layak", res.Verdict, res.Score)
	}
	if res.Source != "rules" {
		t.Fatalf("source = %q, want rules", res.Source)
	}
}

func TestRuleBasedScreening_SLIKHardGate(t *testing.T) {
	sub := domain.ScreeningSubmission{
		Price: 300_000_000,
		Answers: []domain.ScreeningAnswer{
			mkAnswer("scq-income", "Penghasilan", "50000000"), // very high income
			mkAnswer("scq-dp", "DP", "150000000"),
			mkAnswer("scq-slik", "SLIK bersih?", "tidak"), // bad credit history
		},
	}
	res := ruleBasedScreening(nil, sub)
	if res.Verdict != "tidak_layak" {
		t.Fatalf("bad SLIK must gate to tidak_layak, got %q (score %d)", res.Verdict, res.Score)
	}
}

func TestRuleBasedScreening_ReviewWhenNoKeyData(t *testing.T) {
	sub := domain.ScreeningSubmission{
		Answers: []domain.ScreeningAnswer{
			mkAnswer("scq-urgency", "Urgensi", "Sangat mendesak"),
		},
	}
	res := ruleBasedScreening(nil, sub)
	if res.Verdict != "review" {
		t.Fatalf("missing income+SLIK must yield review, got %q", res.Verdict)
	}
}

func TestParseAmountAndBool(t *testing.T) {
	if v := parseAmount("Rp 8.500.000"); v != 8_500_000 {
		t.Fatalf("parseAmount = %.0f, want 8.5jt", v)
	}
	if !parseBool("Ya") || !parseBool("sudah") || parseBool("tidak") {
		t.Fatalf("parseBool mismatch")
	}
}
