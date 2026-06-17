package domain

// ImportSummary is the headline of an approved import, stored in history so the
// admin can see what each upload changed without re-parsing the file.
type ImportSummary struct {
	Leads      int   `json:"leads"`
	ValidLeads int   `json:"validLeads"`
	CV         int   `json:"cv"`
	PV         int   `json:"pv"`
	Booking    int   `json:"booking"`
	Akad       int   `json:"akad"`
	Proses     int   `json:"proses"`
	Batal      int   `json:"batal"`
	CashIn     int64 `json:"cashIn"`
	Issues     int   `json:"issues"`
}

// ImportRecord is one entry in the import history. RolledBack marks an import
// that has already been undone; CanRollback reports whether an undo snapshot is
// still available for it.
type ImportRecord struct {
	ID          string        `json:"id"`
	Time        string        `json:"time"`
	Filename    string        `json:"filename"`
	By          string        `json:"by"`
	Summary     ImportSummary `json:"summary"`
	RolledBack  bool          `json:"rolledBack"`
	CanRollback bool          `json:"canRollback"`
}
