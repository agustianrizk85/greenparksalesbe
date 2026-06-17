package service

import (
	"testing"

	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/repository"
)

// newTestRepo returns an in-memory repository (empty path = no file
// persistence). A fresh store starts EMPTY apart from the annual target.
func newTestRepo(t *testing.T) repository.SalesRepository {
	t.Helper()
	repo, err := repository.NewRepository("")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	return repo
}

func TestEmptyStoreStartsBlank(t *testing.T) {
	svc := New(newTestRepo(t))
	s := svc.Summary()
	if s.Target2026 != 500 {
		t.Errorf("expected default target 500, got %d", s.Target2026)
	}
	if s.Akad != 0 || s.TotalProjects != 0 || s.TotalSalesReps != 0 {
		t.Errorf("expected empty dashboard, got akad=%d projects=%d reps=%d", s.Akad, s.TotalProjects, s.TotalSalesReps)
	}
	if len(svc.Funnel()) != 0 || len(svc.Projects()) != 0 {
		t.Errorf("expected no funnel/projects in a fresh store")
	}
}

func TestSummaryDerivation(t *testing.T) {
	svc := New(newTestRepo(t))
	// Establish a known exec snapshot to derive from.
	if err := svc.SetExec(domain.Exec{Target2026: 500, Booking: 148, Akad: 104, Proses: 44, Batal: 31, RevenueAkad: 70_822_017_000}); err != nil {
		t.Fatalf("SetExec: %v", err)
	}
	s := svc.Summary()

	// 104 akad / 500 target = 20.8%
	if s.Achievement < 20.7 || s.Achievement > 20.9 {
		t.Errorf("achievement out of expected range: %v", s.Achievement)
	}
	if s.GapToTarget != 396 {
		t.Errorf("expected gap 396, got %d", s.GapToTarget)
	}
	// 104 akad + 44 proses = 148 pipeline aktif
	if s.PipelineActive != 148 {
		t.Errorf("expected pipeline 148, got %d", s.PipelineActive)
	}
	// 104 / 148 = 70.3%
	if s.BookingToAkad < 70.2 || s.BookingToAkad > 70.4 {
		t.Errorf("booking->akad out of expected range: %v", s.BookingToAkad)
	}
	// achievement 20.8% with booking->akad 70.3% → risk
	if s.Status != "risk" {
		t.Errorf("expected status risk, got %q", s.Status)
	}
}

func TestProjectByCode(t *testing.T) {
	svc := New(newTestRepo(t))
	if _, err := svc.ProjectByCode("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown project code")
	}
	if _, err := svc.SaveProject(domain.Project{Code: "VERLIM", Name: "Vertihauz Limo-3", Akad: 19, Cat: "utama"}); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	// case-insensitive lookup
	p, err := svc.ProjectByCode("verlim")
	if err != nil {
		t.Fatalf("expected to find project, got %v", err)
	}
	if p.Akad != 19 {
		t.Errorf("expected VERLIM akad 19, got %d", p.Akad)
	}
}

func TestSaveProjectFlowsIntoDashboard(t *testing.T) {
	svc := New(newTestRepo(t))
	before := len(svc.Projects())

	saved, err := svc.SaveProject(domain.Project{Code: "NEW", Name: "New Project", Total: 5, Akad: 2, Cat: "pendukung"})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if saved.EntID == "" {
		t.Fatal("expected generated _id on create")
	}
	if got := svc.Dashboard().Summary.TotalProjects; got != before+1 {
		t.Errorf("dashboard summary should reflect new project: got %d want %d", got, before+1)
	}

	// update in place (same id) must not grow the list
	saved.Akad = 9
	if _, err := svc.SaveProject(saved); err != nil {
		t.Fatalf("update SaveProject: %v", err)
	}
	if got := len(svc.Projects()); got != before+1 {
		t.Errorf("update should not append: got %d want %d", got, before+1)
	}

	if ok, _ := svc.DeleteProject(saved.EntID); !ok {
		t.Fatal("expected delete to succeed")
	}
	if got := len(svc.Projects()); got != before {
		t.Errorf("after delete expected %d projects, got %d", before, got)
	}
}

func TestExecEditUpdatesSummary(t *testing.T) {
	svc := New(newTestRepo(t))
	if err := svc.SetExec(domain.Exec{Target2026: 500, Akad: 250}); err != nil {
		t.Fatalf("SetExec: %v", err)
	}
	if got := svc.Summary().Achievement; got != 50 {
		t.Errorf("expected achievement 50 after akad=250/target=500, got %v", got)
	}
}

func TestSetFunnelRoundTrip(t *testing.T) {
	svc := New(newTestRepo(t))
	in := []domain.FunnelStage{
		{Key: "Leads", Value: 20150, Target: 20150, Owner: "Marketing"},
		{Key: "Purchaser", Value: 153, Target: 110, Owner: "Sales"},
	}
	if err := svc.SetFunnel(in); err != nil {
		t.Fatalf("SetFunnel: %v", err)
	}
	f := svc.Funnel()
	if len(f) != 2 || f[0].Key != "Leads" || f[len(f)-1].Value != 153 {
		t.Errorf("funnel round-trip mismatch: %+v", f)
	}
}
