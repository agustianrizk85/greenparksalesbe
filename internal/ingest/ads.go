package ingest

// mapAds reads META ADS INPUT (header on row 3) and folds Q1/Q2 spend into the
// exec totals and per-project ads, keying on the canonical project code.
func mapAds(rs rows, sd *salesData, res *Result) {
	cProj := rs.col("project")
	cQ1 := rs.col("q1")
	cQ2 := rs.col("q2")
	if cProj < 0 {
		res.addIssue("Kolom Wajib", SevWarning, sheetAds, 0, "kolom 'PROJECT' tidak ditemukan")
		return
	}
	var q1, q2 int64
	for i := range rs.data {
		raw := trim(rs.cell(i, cProj))
		if raw == "" || hasAny(raw, "total") {
			continue
		}
		code, _ := NormalizeProject(raw)
		var a1, a2 int64
		if cQ1 >= 0 {
			a1, _ = ParseRupiah(rs.cell(i, cQ1))
		}
		if cQ2 >= 0 {
			a2, _ = ParseRupiah(rs.cell(i, cQ2))
		}
		q1 += a1
		q2 += a2
		if pa, ok := sd.projects[code]; ok {
			pa.ads += a1 + a2
		}
	}
	res.Preview.Exec.AdsSpentQ1 = q1
	res.Preview.Exec.AdsSpentQ2 = q2
	res.Preview.Exec.AdsSpent = q1 + q2
}
