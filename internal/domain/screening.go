package domain

// This file holds the "Konsumen Screening" entities: a dynamic questionnaire the
// Kadep (sales dept head, admin role) configures, the answers a sales staff fills
// in per prospective buyer, and the resulting AI eligibility verdict ("layak /
// tidak layak membeli rumah"). Staff only input answers — they never edit the
// questions — so the question set is a singleton the admin replaces as a whole,
// while every completed screening is appended as a submission.

// ScreeningFieldType enumerates the input kinds a dynamic question can take.
// The front-end renders the matching control; the value is always carried as a
// string in an answer (numbers/currency formatted client-side).
const (
	FieldText     = "text"     // free short text
	FieldTextarea = "textarea" // free long text
	FieldNumber   = "number"   // plain number
	FieldCurrency = "currency" // Rupiah amount
	FieldBoolean  = "boolean"  // Ya / Tidak
	FieldSelect   = "select"   // one of Options
)

// ScreeningQuestion is one dynamic question in the screening questionnaire. The
// Kadep owns the set (add / edit / reorder / (de)activate); staff answer only the
// Active ones. Category groups questions in the UI; Weight (1..5) hints the
// rule-based fallback how important the factor is when AI scoring is unavailable.
type ScreeningQuestion struct {
	EntID    string   `json:"_id"`
	Order    int      `json:"order"`
	Label    string   `json:"label"`
	Hint     string   `json:"hint,omitempty"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Category string   `json:"category,omitempty"`
	Weight   int      `json:"weight,omitempty"`
	Required bool     `json:"required,omitempty"`
	Active   bool     `json:"active"`
}

func (q ScreeningQuestion) GetID() string { return q.EntID }

// ScreeningAnswer is one staff-supplied answer, snapshotting the question label
// at submit time so a later question edit never rewrites past screenings.
type ScreeningAnswer struct {
	QuestionID string `json:"questionId"`
	Label      string `json:"label"`
	Value      string `json:"value"`
}

// ScreeningResult is the eligibility verdict for a prospective buyer. It comes
// from the AI (OpenRouter) grounded on the questions+answers, or from the
// rule-based fallback when AI is not configured / a call fails.
//
// Verdict is one of:
//
//	layak       — recommended to proceed
//	bersyarat   — eligible with conditions (needs follow-up / documents)
//	tidak_layak — not recommended
//	review      — could not be scored automatically, needs manual review
type ScreeningResult struct {
	Verdict         string   `json:"verdict"`
	Score           int      `json:"score"` // 0..100 kelayakan
	Summary         string   `json:"summary"`
	Strengths       []string `json:"strengths"`
	Risks           []string `json:"risks"`
	Recommendations []string `json:"recommendations"`
	Source          string   `json:"source"`         // "ai" | "rules"
	Note            string   `json:"note,omitempty"` // e.g. why the fallback was used
}

// ScreeningSubmission is one completed screening: who was assessed, the answers
// given, and the verdict. Persisted so the Kadep can review the qualification
// pipeline and staff can revisit their prospects.
type ScreeningSubmission struct {
	EntID     string            `json:"_id"`
	Consumer  string            `json:"consumer"`
	Phone     string            `json:"phone,omitempty"`
	Project   string            `json:"project,omitempty"`
	Unit      string            `json:"unit,omitempty"`
	Price     int64             `json:"price,omitempty"` // harga rumah (Rp)
	Answers   []ScreeningAnswer `json:"answers"`
	Result    ScreeningResult   `json:"result"`
	By        string            `json:"by"`       // staff username (owner)
	ByName    string            `json:"byName"`   // staff display name
	CreatedAt string            `json:"createdAt"` // RFC3339
}

func (s ScreeningSubmission) GetID() string { return s.EntID }

// DefaultScreeningQuestions is the starter questionnaire a fresh store seeds, so
// the feature is usable out of the box before the Kadep customises it. Ordered
// finansial → dokumen → kebutuhan, which is the natural qualification flow.
func DefaultScreeningQuestions() []ScreeningQuestion {
	mk := func(order int, id, label, hint, typ, cat string, weight int, required bool, opts ...string) ScreeningQuestion {
		return ScreeningQuestion{
			EntID: id, Order: order, Label: label, Hint: hint, Type: typ,
			Category: cat, Weight: weight, Required: required, Active: true, Options: opts,
		}
	}
	return []ScreeningQuestion{
		mk(1, "scq-income", "Penghasilan bersih per bulan", "Take-home pay konsumen (dan pasangan bila join income)", FieldCurrency, "Finansial", 5, true),
		mk(2, "scq-otherinstallment", "Total cicilan/utang lain per bulan", "KPR lain, KTA, paylater, cicilan kendaraan, dll", FieldCurrency, "Finansial", 4, true),
		mk(3, "scq-dp", "Kesiapan uang muka (DP)", "Dana tunai yang sudah disiapkan untuk DP", FieldCurrency, "Finansial", 4, true),
		mk(4, "scq-funding", "Rencana sumber dana", "", FieldSelect, "Finansial", 3, true, "KPR", "Cash Bertahap", "Cash Keras"),
		mk(5, "scq-jobtype", "Status pekerjaan", "", FieldSelect, "Finansial", 3, true, "Karyawan Tetap", "Karyawan Kontrak", "Wiraswasta", "Profesional", "Lainnya"),
		mk(6, "scq-worklength", "Lama bekerja / usaha berjalan (tahun)", "Untuk syarat KPR biasanya minimal 2 tahun", FieldNumber, "Finansial", 2, false),
		mk(7, "scq-slik", "Riwayat kredit (BI Checking/SLIK) bersih?", "Tidak ada tunggakan/kredit macet", FieldBoolean, "Dokumen", 5, true),
		mk(8, "scq-docs", "Dokumen KPR sudah lengkap?", "KTP, KK, slip gaji, rekening koran, NPWP", FieldBoolean, "Dokumen", 3, true),
		mk(9, "scq-urgency", "Tingkat urgensi membeli", "", FieldSelect, "Kebutuhan", 2, false, "Sangat mendesak", "Dalam 3 bulan", "Masih survei"),
		mk(10, "scq-purpose", "Tujuan pembelian", "", FieldSelect, "Kebutuhan", 1, false, "Tempat tinggal", "Investasi", "Lainnya"),
		mk(11, "scq-notes", "Catatan tambahan dari sales", "Konteks lain yang relevan untuk penilaian", FieldTextarea, "Kebutuhan", 1, false),
	}
}
