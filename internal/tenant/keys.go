package tenant

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type TenantKey struct {
	TenantID  string    `json:"tenant_id"`
	Provider  string    `json:"provider"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	db        *sql.DB
	masterKey []byte
}

func NewStore(dsn, adminKey string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, masterKey: deriveMask(adminKey)}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func deriveMask(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

func (s *Store) encrypt(plaintext string) string {
	mask := s.masterKey
	input := []byte(plaintext)
	output := make([]byte, len(input))
	for i := range input {
		output[i] = input[i] ^ mask[i%len(mask)]
	}
	return base64.StdEncoding.EncodeToString(output)
}

func (s *Store) decrypt(ciphertext string) (string, error) {
	mask := s.masterKey
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	output := make([]byte, len(data))
	for i := range data {
		output[i] = data[i] ^ mask[i%len(mask)]
	}
	return string(output), nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(128) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    encrypted_key TEXT NOT NULL,
    key_hash VARCHAR(128) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tenant_id, provider)
);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_tenant ON tenant_api_keys(tenant_id);
`)
	return err
}

func (s *Store) PutKey(ctx context.Context, tenantID, provider, apiKey string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(apiKey)))
	enc := s.encrypt(apiKey)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO tenant_api_keys (tenant_id, provider, encrypted_key, key_hash, is_active, updated_at)
VALUES ($1,$2,$3,$4,true,ON CONFLICT (tenant_id, provider) DO UPDATE SET encrypted_key=$3, key_hash=$4, is_active=true, updated_at=NOW())
`, tenantID, provider, enc, hash)
	return err
}

func (s *Store) GetKey(ctx context.Context, tenantID, provider string) (string, error) {
	var enc string
	err := s.db.QueryRowContext(ctx, `
SELECT encrypted_key FROM tenant_api_keys WHERE tenant_id=$1 AND provider=$2 AND is_active=true
`, tenantID, provider).Scan(&enc)
	if err != nil {
		return "", err
	}
	return s.decrypt(enc)
}

func (s *Store) DeleteKey(ctx context.Context, tenantID, provider string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tenant_api_keys WHERE tenant_id=$1 AND provider=$2`, tenantID, provider)
	return err
}

func (s *Store) ListKeys(ctx context.Context, tenantID string) ([]TenantKey, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tenant_id, provider, is_active, created_at FROM tenant_api_keys WHERE tenant_id=$1 ORDER BY provider
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []TenantKey
	for rows.Next() {
		var k TenantKey
		if err := rows.Scan(&k.TenantID, &k.Provider, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) ListAllKeys(ctx context.Context) ([]TenantKey, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tenant_id, provider, is_active, created_at FROM tenant_api_keys ORDER BY tenant_id, provider
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []TenantKey
	for rows.Next() {
		var k TenantKey
		if err := rows.Scan(&k.TenantID, &k.Provider, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
