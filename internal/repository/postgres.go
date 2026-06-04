package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql driver ("pgx")
)

// pgPersister stores the whole dashboard state as a single JSONB row in
// PostgreSQL (table sales_state, id=1). It mirrors the file persister so the
// repository logic is unchanged; only the storage backend differs.
type pgPersister struct {
	db *sql.DB
}

func newPGPersister(dsn string) (*pgPersister, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS sales_state (
		id   int   PRIMARY KEY,
		data jsonb NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &pgPersister{db: db}, nil
}

// load reads the state row; on first run it seeds the default state and saves it.
func (p *pgPersister) load() (*state, error) {
	var raw []byte
	err := p.db.QueryRow(`SELECT data FROM sales_state WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		st := seedState()
		if err := p.save(st); err != nil {
			return nil, err
		}
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	st := &state{}
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	return st, nil
}

// save upserts the whole state into the single row.
func (p *pgPersister) save(st *state) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(
		`INSERT INTO sales_state(id, data) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`, string(b))
	return err
}
