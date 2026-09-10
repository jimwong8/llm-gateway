package governance

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func (s *Store) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.bootstrap(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) bootstrap(ctx context.Context) error {
	for _, migration := range migrationFiles() {
		raw, err := readFile(migration)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migration, err)
		}
		for _, stmt := range splitSQLStatements(raw) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("exec migration %s: %w", migration, err)
			}
		}
	}
	return nil
}

func (s *Store) TableExists(ctx context.Context, table string) bool {
	table = strings.TrimSpace(table)
	if table == "" {
		return false
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1
	)`, table).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func migrationFiles() []string {
	_, currentFile, _, _ := runtime.Caller(0)
	base := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "db", "migrations"))
	return []string{
		filepath.Join(base, "004_model_governance_init.sql"),
		filepath.Join(base, "005_model_governance_constraints.sql"),
		filepath.Join(base, "006_model_governance_runtime.sql"),
		filepath.Join(base, "007_model_governance_evaluations.sql"),
		filepath.Join(base, "008_model_governance_distribution_and_drift.sql"),
		filepath.Join(base, "009_model_governance_rollbacks.sql"),
	}
}

func readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func splitSQLStatements(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}
