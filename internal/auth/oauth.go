package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"time"
)

type OAuthBinding struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	AccessToken    string    `json:"-"`
	RefreshToken   string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *Store) CreateOAuthBinding(ctx context.Context, userID int64, provider, providerUserID, accessToken, refreshToken, encryptionKey string) (*OAuthBinding, error) {
	encryptedAccess, err := encryptToken(accessToken, encryptionKey)
	if err != nil {
		return nil, err
	}
	encryptedRefresh, err := encryptToken(refreshToken, encryptionKey)
	if err != nil {
		return nil, err
	}

	var b OAuthBinding
	err = s.db.QueryRowContext(ctx, `
INSERT INTO oauth_bindings (user_id, provider, provider_user_id, access_token, refresh_token)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, provider, provider_user_id, access_token, refresh_token, created_at, updated_at`,
		userID, provider, providerUserID, encryptedAccess, encryptedRefresh,
	).Scan(&b.ID, &b.UserID, &b.Provider, &b.ProviderUserID, &b.AccessToken, &b.RefreshToken, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	b.AccessToken, _ = decryptToken(b.AccessToken, encryptionKey)
	b.RefreshToken, _ = decryptToken(b.RefreshToken, encryptionKey)
	return &b, nil
}

func (s *Store) GetOAuthBindingByProvider(ctx context.Context, provider, providerUserID, encryptionKey string) (*OAuthBinding, error) {
	var b OAuthBinding
	err := s.db.QueryRowContext(ctx, `
SELECT id, user_id, provider, provider_user_id, access_token, refresh_token, created_at, updated_at
FROM oauth_bindings WHERE provider = $1 AND provider_user_id = $2`,
		provider, providerUserID,
	).Scan(&b.ID, &b.UserID, &b.Provider, &b.ProviderUserID, &b.AccessToken, &b.RefreshToken, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	b.AccessToken, _ = decryptToken(b.AccessToken, encryptionKey)
	b.RefreshToken, _ = decryptToken(b.RefreshToken, encryptionKey)
	return &b, nil
}

func (s *Store) ListOAuthBindingsByUserID(ctx context.Context, userID int64, encryptionKey string) ([]OAuthBinding, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, provider, provider_user_id, access_token, refresh_token, created_at, updated_at
FROM oauth_bindings WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []OAuthBinding
	for rows.Next() {
		var b OAuthBinding
		if err := rows.Scan(&b.ID, &b.UserID, &b.Provider, &b.ProviderUserID, &b.AccessToken, &b.RefreshToken, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.AccessToken, _ = decryptToken(b.AccessToken, encryptionKey)
		b.RefreshToken, _ = decryptToken(b.RefreshToken, encryptionKey)
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

func (s *Store) DeleteOAuthBinding(ctx context.Context, userID int64, provider string) error {
	result, err := s.db.ExecContext(ctx, `
DELETE FROM oauth_bindings WHERE user_id = $1 AND provider = $2`, userID, provider)
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

func (s *Store) GetOrCreateUserByOAuth(ctx context.Context, provider, providerUserID, email, username, accessToken, refreshToken, encryptionKey string) (*User, *OAuthBinding, error) {
	binding, err := s.GetOAuthBindingByProvider(ctx, provider, providerUserID, encryptionKey)
	if err == nil && binding != nil {
		user, err := s.GetUserByID(ctx, binding.UserID)
		if err != nil {
			return nil, nil, err
		}
		return user, binding, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}

	existingUser, err := s.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	if existingUser != nil {
		binding, err := s.CreateOAuthBinding(ctx, existingUser.ID, provider, providerUserID, accessToken, refreshToken, encryptionKey)
		if err != nil {
			return nil, nil, err
		}
		return existingUser, binding, nil
	}

	user, err := s.CreateUser(ctx, email, username, "__oauth__")
	if err != nil {
		return nil, nil, err
	}

	binding, err = s.CreateOAuthBinding(ctx, user.ID, provider, providerUserID, accessToken, refreshToken, encryptionKey)
	if err != nil {
		return nil, nil, err
	}

	return user, binding, nil
}

func encryptToken(plaintext, key string) (string, error) {
	if key == "" {
		return plaintext, nil
	}
	keyBytes := deriveKey(key)
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptToken(encoded, key string) (string, error) {
	if key == "" {
		return encoded, nil
	}
	keyBytes := deriveKey(key)
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func deriveKey(secret string) []byte {
	h := make([]byte, 32)
	copy(h, []byte(secret))
	return h
}
