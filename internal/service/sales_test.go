package service

import (
	"testing"

	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/repository"
)

// newTestRepo returns an in-memory repository (empty path = no file persistence).
func newTestRepo(t *testing.T) repository.SalesRepository {
	t.Helper()
	repo, err := repository.NewRepository("")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	return repo
}

func TestSummaryDerivation(t *testing.T) {
	svc := New(newTestRepo(t))
	s := svc.Summary()

	if s.Target2026 != 500 {
		t.Errorf("expected target 500, got %d", s.Target2026)
	}
	// 103 akad / 500 target = 20.6%
	if s.Achievement < 20.5 || s.Achievement > 20.7 {
		t.Errorf("achievement out of expected range: %v", s.Achievement)
	}
	if s.GapToTarget != 397 {
		t.Errorf("expected gap 397, got %d", s.GapToTarget)
	}
	// 103 akad + 41 proses = 144 pipeline aktif
	if s.PipelineActive != 144 {
		t.Errorf("expected pipeline 144, got %d", s.PipelineActive)
	}
	// 103 / 144 = 71.5%
	if s.BookingToAkad < 71.4 || s.BookingToAkad > 71.6 {
		t.Errorf("booking->akad out of expected range: %v", s.BookingToAkad)
	}
	if s.TotalProjects != 12 {
		t.Errorf("expected 12 projects, got %d", s.TotalProjects)
	}
	if s.TotalSalesReps != 21 {
		t.Errorf("expected 21 sales reps, got %d", s.TotalSalesReps)
	}
	// achievement 20.6% with booking->akad 71.5% → risk
	if s.Status != "risk" {
		t.Errorf("expected status risk, got %q", s.Status)
	}
}

func TestProjectByCode(t *testing.T) {
	svc := New(newTestRepo(t))
	if _, err := svc.ProjectByCode("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown project code")
	}
	// case-insensitive lookup
	p, err := svc.ProjectByCode("verlim3")
	if err != nil {
		t.Fatalf("expected to find seeded project, got %v", err)
	}
	if p.Akad != 19 {
		t.Errorf("expected VERLIM3 akad 19, got %d", p.Akad)
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
	e := svc.Exec()
	e.Akad = 250
	if err := svc.SetExec(e); err != nil {
		t.Fatalf("SetExec: %v", err)
	}
	if got := svc.Summary().Achievement; got != 50 {
		t.Errorf("expected achievement 50 after akad=250/target=500, got %v", got)
	}
}

func TestFunnelStandards(t *testing.T) {
	svc := New(newTestRepo(t))
	f := svc.Funnel()
	// BR-9: leads-only funnel of 5 stages ending at Purchaser (not Booking/Cash-In).
	if len(f) != 5 {
		t.Fatalf("expected 5 funnel stages, got %d", len(f))
	}
	if f[0].Key != "Leads" || f[0].Std != nil {
		t.Errorf("expected first stage Leads with nil std")
	}
	last := f[len(f)-1]
	if last.Key != "Purchaser" || last.IsMoney {
		t.Errorf("expected last stage Purchaser (non-money), got %q", last.Key)
	}
	if last.Value != 74 {
		t.Errorf("expected Purchaser = 74 per BR-9, got %d", last.Value)
	}
}
