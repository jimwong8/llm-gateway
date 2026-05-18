package auth

import (
	"context"
	"database/sql"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         int       `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	RPMILimit  int        `json:"rpm_limit"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateUser(ctx context.Context, email, username, passwordHash string) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
INSERT INTO users (email, username, password_hash, role, status)
VALUES ($1, $2, $3, 1, 'active')
RETURNING id, email, username, password_hash, role, status, created_at, updated_at`,
		email, username, passwordHash,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT id, email, username, password_hash, role, status, created_at, updated_at
FROM users WHERE email = $1 AND status = 'active'`, email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT id, email, username, password_hash, role, status, created_at, updated_at
FROM users WHERE id = $1 AND status = 'active'`, id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) CreateAPIKey(ctx context.Context, userID int64, keyPrefix, keyHash, name string) (*APIKey, error) {
	var key APIKey
	err := s.db.QueryRowContext(ctx, `
INSERT INTO user_api_keys (user_id, key_prefix, key_hash, name, status)
VALUES ($1, $2, $3, $4, 'active')
RETURNING id, user_id, key_prefix, key_hash, name, status, rpm_limit, last_used_at, created_at, updated_at`,
		userID, keyPrefix, keyHash, name,
	).Scan(&key.ID, &key.UserID, &key.KeyPrefix, &key.KeyHash, &key.Name, &key.Status, &key.RPMILimit, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, key_prefix, key_hash, name, status, rpm_limit, last_used_at, created_at, updated_at
FROM user_api_keys WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Status, &k.RPMILimit, &k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) RevokeAPIKey(ctx context.Context, userID, keyID int64) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE user_api_keys SET status = 'revoked', updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND status = 'active'`, keyID, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
	var k APIKey
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, key_prefix, key_hash, name, status, rpm_limit, last_used_at, created_at, updated_at
FROM user_api_keys WHERE key_prefix = $1 AND status = 'active'`, prefix,
	).Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Status, &k.RPMILimit, &k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *Store) UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE user_api_keys SET last_used_at = $1, updated_at = $1 WHERE id = $2`, now, keyID)
	return err
}

func (s *Store) GetAPIKeyByID(ctx context.Context, keyID int64) (*APIKey, error) {
	var k APIKey
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, key_prefix, key_hash, name, status, rpm_limit, last_used_at, created_at, updated_at
FROM user_api_keys WHERE id = $1`, keyID,
	).Scan(&k.ID, &k.UserID, &k.KeyPrefix, &k.KeyHash, &k.Name, &k.Status, &k.RPMILimit, &k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}
