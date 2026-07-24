package domain

// This file holds the "SKP" (Surat Konfirmasi Pesanan — unit booking
// confirmation letter) entities: a per-project master template the Kadep
// maintains (project name/address, booking-fee bank account) so sales staff
// don't retype the fixed parts, and each issued SKP — a snapshot of a buyer's
// data + the unit they booked, ready to print for signature.

// SkpProjectTemplate is the fixed data for one project, reused when a new SKP
// is created for that project. The bank/account fields are the developer's
// receiving account for the booking fee.
type SkpProjectTemplate struct {
	EntID          string `json:"_id"`
	ProjectName    string `json:"projectName"`
	ProjectAddress string `json:"projectAddress"`
	AccountHolder  string `json:"accountHolder"` // e.g. "PT Greenpark Properti Utama"
	BankName       string `json:"bankName"`      // e.g. "BSI (Bank Syariah Indonesia)"
	BankAccount    string `json:"bankAccount"`
	BankCode       string `json:"bankCode"`
	BookingFee     int64  `json:"bookingFee"`    // default booking fee (Rp)
	MarketingCity  string `json:"marketingCity"` // kota kantor pemasaran (tanda tangan SKP)
	Active         bool   `json:"active"`
}

func (t SkpProjectTemplate) GetID() string { return t.EntID }

// SkpAddress is a reusable address block — KTP / domisili address share the
// same shape (jalan, RT/RW, kelurahan, kecamatan, kota).
type SkpAddress struct {
	Alamat    string `json:"alamat"`
	RtRw      string `json:"rtRw"`
	Kelurahan string `json:"kelurahan"`
	Kecamatan string `json:"kecamatan"`
	Kota      string `json:"kota"`
}

// Skp is one issued Surat Konfirmasi Pesanan: buyer data + booked unit,
// snapshotting the project's bank/address at creation time so a later
// template edit never rewrites an already-issued SKP.
type Skp struct {
	EntID string `json:"_id"`
	Nomor string `json:"nomor,omitempty"`

	// ---- Data Pemesan ----
	Nama           string     `json:"nama"`
	NoKTP          string     `json:"noKtp"`
	AlamatKTP      SkpAddress `json:"alamatKtp"`
	AlamatDomisili SkpAddress `json:"alamatDomisili"`
	Agama          string     `json:"agama,omitempty"`
	StatusKawin    string     `json:"statusKawin,omitempty"`
	Pekerjaan      string     `json:"pekerjaan,omitempty"`
	AlamatKantor   SkpAddress `json:"alamatKantor"`
	NoHP           string     `json:"noHp"`
	NoTelpKantor   string     `json:"noTelpKantor,omitempty"`
	Email          string     `json:"email,omitempty"`
	SumberInfo     string     `json:"sumberInfo,omitempty"`

	// ---- Pembayaran booking fee ----
	BookingFee    int64  `json:"bookingFee"`
	BookingFeeVia string `json:"bookingFeeVia"` // "transfer" | "tunai"

	// ---- Data Unit ----
	ProjectTemplateID string `json:"projectTemplateId,omitempty"`
	NamaProyek        string `json:"namaProyek"`
	AlamatProyek      string `json:"alamatProyek"`
	TypeUnit          string `json:"typeUnit"`
	BlokNoUnit        string `json:"blokNoUnit"`
	LuasTanah         string `json:"luasTanah,omitempty"`
	LuasBangunan      string `json:"luasBangunan,omitempty"`
	HargaJual         int64  `json:"hargaJual"`
	DownPayment       int64  `json:"downPayment"`
	Promo             string `json:"promo,omitempty"`
	CaraBayar         string `json:"caraBayar"` // "kpr" | "cash_keras" | "cash_bertahap"
	AlasanPembelian   string `json:"alasanPembelian,omitempty"`

	// ---- Rekening (snapshot dari template proyek) ----
	AccountHolder string `json:"accountHolder"`
	BankName      string `json:"bankName"`
	BankAccount   string `json:"bankAccount"`
	BankCode      string `json:"bankCode"`

	// ---- Tanda tangan ----
	MarketingName   string `json:"marketingName,omitempty"`
	FinanceName     string `json:"financeName,omitempty"`
	SignCity        string `json:"signCity,omitempty"`
	SignDate        string `json:"signDate,omitempty"` // tanggal SKP ditandatangani (YYYY-MM-DD), kosong = draft

	By        string `json:"by"` // staff username pembuat
	ByName    string `json:"byName"`
	CreatedAt string `json:"createdAt"` // RFC3339
}

func (s Skp) GetID() string { return s.EntID }

// Unit booking status — prevents selling/booking the same unit twice. Pipeline
// order: tersedia -> booked (DP/booking fee masuk) -> akad (akad kredit/jual-beli
// ditandatangani) -> terjual (lunas/selesai), dengan "batal" di titik manapun
// kalau transaksinya batal (mis. gagal DP/KPR).
const (
	UnitTersedia = "tersedia"
	UnitBooked   = "booked"
	UnitAkad     = "akad"
	UnitTerjual  = "terjual"
	UnitBatal    = "batal"
)

// UnitBooking tracks one unit's availability within a project. Created/updated
// manually by the Kadep (initial inventory) and automatically flipped to
// "booked" (linked to the SKP) whenever a new SKP is issued for that unit.
type UnitBooking struct {
	EntID      string `json:"_id"`
	NamaProyek string `json:"namaProyek"`
	TypeUnit   string `json:"typeUnit,omitempty"`
	BlokNoUnit string `json:"blokNoUnit"`
	Status     string `json:"status"` // tersedia | booked | akad | terjual | batal
	SkpID      string `json:"skpId,omitempty"`
	Note       string `json:"note,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
}

func (u UnitBooking) GetID() string { return u.EntID }
