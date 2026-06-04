package repository

import (
	"os"
	"testing"

	"greenpark/sales/internal/domain"
)

// TestPostgresStateIntegration runs against a real PostgreSQL when
// TEST_DATABASE_URL is set; otherwise it is skipped.
func TestPostgresStateIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}

	repo, err := NewPostgresRepository(dsn)
	if err != nil {
		t.Fatalf("NewPostgresRepository: %v", err)
	}

	// Seed loaded on first run.
	if len(repo.Projects()) == 0 {
		t.Fatal("expected seeded projects")
	}
	if _, err := repo.UserByUsername("admin"); err != nil {
		t.Fatalf("seeded admin user missing: %v", err)
	}
	before := len(repo.Projects())

	// Create a project.
	saved, err := repo.SaveProject(domain.Project{Code: "ZZTEST", Name: "PG Persist"})
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if saved.EntID == "" {
		t.Fatal("SaveProject did not assign an id")
	}

	// Reopen (simulates a server restart) → data persisted.
	repo2, err := NewPostgresRepository(dsn)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(repo2.Projects()); got != before+1 {
		t.Fatalf("after restart projects = %d, want %d", got, before+1)
	}
	if _, err := repo2.ProjectByCode("ZZTEST"); err != nil {
		t.Errorf("created project not persisted: %v", err)
	}

	// Cleanup.
	if ok, err := repo2.DeleteProject(saved.EntID); err != nil || !ok {
		t.Errorf("DeleteProject ok=%v err=%v", ok, err)
	}
}
