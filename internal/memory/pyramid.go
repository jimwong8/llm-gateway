package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Atom struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	Content       string     `json:"content"`
	Tags          []string   `json:"tags"`
	Source        string     `json:"source"`
	Importance    float64    `json:"importance"`
	AccessCount   int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Scenario struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	ChatSessionID *int64    `json:"chat_session_id,omitempty"`
	AtomIDs       []int64   `json:"atom_ids"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Persona struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	Preferences       string    `json:"preferences"`
	Interests         []string  `json:"interests"`
	CommunicationStyle string   `json:"communication_style"`
	ExpertiseAreas    []string  `json:"expertise_areas"`
	Summary           string    `json:"summary"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PyramidStore interface {
	CreateAtom(ctx context.Context, userID int64, content string, tags []string, source string) (*Atom, error)
	ListAtoms(ctx context.Context, userID int64, limit int) ([]Atom, error)
	SearchAtoms(ctx context.Context, userID int64, query string, limit int) ([]Atom, error)
	DeleteAtom(ctx context.Context, userID, atomID int64) error

	CreateScenario(ctx context.Context, userID int64, title, summary string, sessionID *int64, atomIDs []int64) (*Scenario, error)
	ListScenarios(ctx context.Context, userID int64, limit int) ([]Scenario, error)

	GetPersona(ctx context.Context, userID int64) (*Persona, error)
	SavePersona(ctx context.Context, userID int64, preferences, interests, expertise, summary string) (*Persona, error)
}

type sqlPyramidStore struct {
	db *sql.DB
}

func NewPyramidStore(db *sql.DB) PyramidStore {
	return &sqlPyramidStore{db: db}
}

func (s *sqlPyramidStore) CreateAtom(ctx context.Context, userID int64, content string, tags []string, source string) (*Atom, error) {
	if source == "" {
		source = "chat"
	}
	var a Atom
	err := s.db.QueryRowContext(ctx, `
INSERT INTO memory_atoms (user_id, content, tags, source)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, content, tags, source, importance, access_count, last_accessed_at, created_at, updated_at`,
		userID, content, pqArray(tags), source,
	).Scan(&a.ID, &a.UserID, &a.Content, pqArrayScanner(&a.Tags), &a.Source, &a.Importance, &a.AccessCount, &a.LastAccessedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create atom: %w", err)
	}
	return &a, nil
}

func (s *sqlPyramidStore) ListAtoms(ctx context.Context, userID int64, limit int) ([]Atom, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, content, tags, source, importance, access_count, last_accessed_at, created_at, updated_at
FROM memory_atoms WHERE user_id = $1 ORDER BY importance DESC, created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAtoms(rows)
}

func (s *sqlPyramidStore) SearchAtoms(ctx context.Context, userID int64, query string, limit int) ([]Atom, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, content, tags, source, importance, access_count, last_accessed_at, created_at, updated_at
FROM memory_atoms
WHERE user_id = $1 AND (content ILIKE $2 OR $3 = ANY(tags))
ORDER BY importance DESC, access_count DESC LIMIT $4`,
		userID, "%"+query+"%", query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAtoms(rows)
}

func (s *sqlPyramidStore) DeleteAtom(ctx context.Context, userID, atomID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memory_atoms WHERE id = $1 AND user_id = $2`, atomID, userID)
	return err
}

func (s *sqlPyramidStore) CreateScenario(ctx context.Context, userID int64, title, summary string, sessionID *int64, atomIDs []int64) (*Scenario, error) {
	var sc Scenario
	err := s.db.QueryRowContext(ctx, `
INSERT INTO memory_scenarios (user_id, title, summary, chat_session_id, atom_ids)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, title, summary, chat_session_id, atom_ids, created_at, updated_at`,
		userID, title, summary, sessionID, pqArrayInt(atomIDs),
	).Scan(&sc.ID, &sc.UserID, &sc.Title, &sc.Summary, &sc.ChatSessionID, pqArrayIntScanner(&sc.AtomIDs), &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create scenario: %w", err)
	}
	return &sc, nil
}

func (s *sqlPyramidStore) ListScenarios(ctx context.Context, userID int64, limit int) ([]Scenario, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, title, summary, chat_session_id, atom_ids, created_at, updated_at
FROM memory_scenarios WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scenarios []Scenario
	for rows.Next() {
		var sc Scenario
		if err := rows.Scan(&sc.ID, &sc.UserID, &sc.Title, &sc.Summary, &sc.ChatSessionID, pqArrayIntScanner(&sc.AtomIDs), &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, sc)
	}
	return scenarios, rows.Err()
}

func (s *sqlPyramidStore) GetPersona(ctx context.Context, userID int64) (*Persona, error) {
	var p Persona
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, preferences::text, interests, communication_style, expertise_areas, summary, created_at, updated_at
FROM memory_personas WHERE user_id = $1`, userID,
	).Scan(&p.ID, &p.UserID, &p.Preferences, pqArrayScanner(&p.Interests), &p.CommunicationStyle, pqArrayScanner(&p.ExpertiseAreas), &p.Summary, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *sqlPyramidStore) SavePersona(ctx context.Context, userID int64, preferences, interests, expertise, summary string) (*Persona, error) {
	var p Persona
	err := s.db.QueryRowContext(ctx, `
INSERT INTO memory_personas (user_id, preferences, interests, expertise_areas, summary)
VALUES ($1, $2::jsonb, $3::text[], $4::text[], $5)
ON CONFLICT (user_id) DO UPDATE SET
  preferences = EXCLUDED.preferences,
  interests = EXCLUDED.interests,
  expertise_areas = EXCLUDED.expertise_areas,
  summary = EXCLUDED.summary,
  updated_at = NOW()
RETURNING id, user_id, preferences::text, interests, communication_style, expertise_areas, summary, created_at, updated_at`,
		userID, preferences, pqArrayStr(interests), pqArrayStr(expertise), summary,
	).Scan(&p.ID, &p.UserID, &p.Preferences, pqArrayScanner(&p.Interests), &p.CommunicationStyle, pqArrayScanner(&p.ExpertiseAreas), &p.Summary, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("save persona: %w", err)
	}
	return &p, nil
}

func scanAtoms(rows *sql.Rows) ([]Atom, error) {
	var atoms []Atom
	for rows.Next() {
		var a Atom
		if err := rows.Scan(&a.ID, &a.UserID, &a.Content, pqArrayScanner(&a.Tags), &a.Source, &a.Importance, &a.AccessCount, &a.LastAccessedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		atoms = append(atoms, a)
	}
	return atoms, rows.Err()
}

func pqArray(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}
	result := "{"
	for i, s := range arr {
		if i > 0 {
			result += ","
		}
		result += `"` + s + `"`
	}
	return result + "}"
}

func pqArrayInt(arr []int64) string {
	if len(arr) == 0 {
		return "{}"
	}
	result := "{"
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", v)
	}
	return result + "}"
}

func pqArrayScanner(arr *[]string) *pqArrayScannerImpl {
	*arr = nil
	return &pqArrayScannerImpl{arr: arr}
}

type pqArrayScannerImpl struct {
	arr *[]string
}

func (s *pqArrayScannerImpl) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case string:
		*s.arr = parsePGArray(v)
	case []byte:
		*s.arr = parsePGArray(string(v))
	}
	return nil
}

func pqArrayIntScanner(arr *[]int64) *pqArrayIntScannerImpl {
	*arr = nil
	return &pqArrayIntScannerImpl{arr: arr}
}

type pqArrayIntScannerImpl struct {
	arr *[]int64
}

func (s *pqArrayIntScannerImpl) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	switch v := src.(type) {
	case string:
		*s.arr = parsePGIntArray(v)
	case []byte:
		*s.arr = parsePGIntArray(string(v))
	}
	return nil
}

func pqArrayStr(s string) string {
	if s == "" {
		return "{}"
	}
	return pqArray([]string{s})
}

func parsePGArray(s string) []string {
	if s == "{}" || s == "" {
		return nil
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}
	parts := splitPGArray(s)
	return parts
}

func parsePGIntArray(s string) []int64 {
	if s == "{}" || s == "" {
		return nil
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}
	parts := splitPGArray(s)
	var result []int64
	for _, p := range parts {
		var v int64
		fmt.Sscanf(p, "%d", &v)
		result = append(result, v)
	}
	return result
}

func splitPGArray(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, unquote(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, unquote(s[start:]))
	}
	return parts
}

func unquote(s string) string {
	s = trimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
