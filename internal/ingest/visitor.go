package ingest

// mapVisitor counts the MASTER DATA_VISITOR rows. Per the funnel rule, visitor
// data is WALK-IN / UNDANGAN only and feeds the Event panel — it never enters
// the main Leads funnel.
func mapVisitor(rs rows, res *Result) {
	cName := rs.col("name")
	if cName < 0 {
		return
	}
	n := 0
	for i := range rs.data {
		if trim(rs.cell(i, cName)) != "" {
			n++
		}
	}
	res.Headline.VisitorWalkIns = n
}
