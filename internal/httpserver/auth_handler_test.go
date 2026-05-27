package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/auth"
	"llm-gateway/gateway/internal/config"
)

// ─── mock userStore ───────────────────────────────────────────────────────────

type authMockStore struct {
	users             map[int64]*auth.User
	usersByEmail      map[string]*auth.User
	apiKeys           map[int64]*auth.APIKey
	nextUserID        int64
	nextKeyID         int64
	createUserErr     error
	getUserByEmailErr error
	getUserByIDErr    error
	createAPIKeyErr   error
	listAPIKeysErr    error
	revokeAPIKeyErr   error
}

func newAuthMockStore() *authMockStore {
	return &authMockStore{
		users:        make(map[int64]*auth.User),
		usersByEmail: make(map[string]*auth.User),
		apiKeys:      make(map[int64]*auth.APIKey),
		nextUserID:   1,
		nextKeyID:    1,
	}
}

func (m *authMockStore) CreateUser(_ context.Context, email, username, passwordHash string) (*auth.User, error) {
	if m.createUserErr != nil {
		return nil, m.createUserErr
	}
	if _, exists := m.usersByEmail[email]; exists {
		return nil, errors.New("duplicate key value violates unique constraint")
	}
	id := m.nextUserID
	m.nextUserID++
	now := time.Now().UTC().Truncate(time.Second)
	u := &auth.User{
		ID:           id,
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         1,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.users[id] = u
	m.usersByEmail[email] = u
	return u, nil
}

func (m *authMockStore) GetUserByEmail(_ context.Context, email string) (*auth.User, error) {
	if m.getUserByEmailErr != nil {
		return nil, m.getUserByEmailErr
	}
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *authMockStore) GetUserByID(_ context.Context, id int64) (*auth.User, error) {
	if m.getUserByIDErr != nil {
		return nil, m.getUserByIDErr
	}
	u, ok := m.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *authMockStore) CreateAPIKey(_ context.Context, userID int64, keyPrefix, keyHash, name string) (*auth.APIKey, error) {
	if m.createAPIKeyErr != nil {
		return nil, m.createAPIKeyErr
	}
	id := m.nextKeyID
	m.nextKeyID++
	now := time.Now().UTC().Truncate(time.Second)
	k := &auth.APIKey{
		ID:        id,
		UserID:    userID,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		Name:      name,
		Status:    "active",
		RPMILimit: 60,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.apiKeys[id] = k
	return k, nil
}

func (m *authMockStore) ListAPIKeys(_ context.Context, userID int64) ([]auth.APIKey, error) {
	if m.listAPIKeysErr != nil {
		return nil, m.listAPIKeysErr
	}
	var keys []auth.APIKey
	for _, k := range m.apiKeys {
		if k.UserID == userID {
			keys = append(keys, *k)
		}
	}
	return keys, nil
}

func (m *authMockStore) RevokeAPIKey(_ context.Context, userID, keyID int64) error {
	if m.revokeAPIKeyErr != nil {
		return m.revokeAPIKeyErr
	}
	k, ok := m.apiKeys[keyID]
	if !ok || k.UserID != userID {
		return sql.ErrNoRows
	}
	k.Status = "revoked"
	return nil
}

func (m *authMockStore) GetAPIKeyByPrefix(_ context.Context, prefix string) (*auth.APIKey, error) {
	for _, k := range m.apiKeys {
		if k.KeyPrefix == prefix {
			return k, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *authMockStore) GetAPIKeyByID(_ context.Context, keyID int64) (*auth.APIKey, error) {
	k, ok := m.apiKeys[keyID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return k, nil
}

func (m *authMockStore) UpdateAPIKeyLastUsed(_ context.Context, keyID int64) error {
	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newTestAuthServer(store *authMockStore) *Server {
	return &Server{
		userStore: store,
		cfg:       config.Config{JWTSecret: testJWTSecret},
	}
}

func signupBody(email, username, password string) string {
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"username": username,
		"password": password,
	})
	return string(body)
}

func loginBody(email, password string) string {
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	return string(body)
}

// ─── authSignup tests ────────────────────────────────────────────────────────

func TestAuthSignup_Success(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("newuser@example.com", "newuser", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Token string         `json:"token"`
		User  map[string]any `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if resp.User["email"] != "newuser@example.com" {
		t.Fatalf("expected email 'newuser@example.com', got %v", resp.User["email"])
	}
	if resp.User["username"] != "newuser" {
		t.Fatalf("expected username 'newuser', got %v", resp.User["username"])
	}
}

func TestAuthSignup_WeakPassword_OnlyDigits(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("user@example.com", "user1", "12345678")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthSignup_PasswordTooShort(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("user@example.com", "user1", "Ab1!")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthSignup_InvalidEmail(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("not-an-email", "user1", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthSignup_EmptyFields(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	tests := []struct {
		name    string
		body    string
	}{
		{"empty email", signupBody("", "user1", "Str0ng!Pass")},
		{"empty username", signupBody("user@example.com", "", "Str0ng!Pass")},
		{"empty password", signupBody("user@example.com", "user1", "")},
		{"all empty", signupBody("", "", "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAuthSignup_DuplicateEmail(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	// 第一次注册成功
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("dup@example.com", "user1", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("first signup: expected 201, got %d", rr.Code)
	}

	// 重复注册
	req = httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("dup@example.com", "user2", "An0ther!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", rr.Code, rr.Body.String())
	}

	// 验证返回模糊错误消息
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	errMsg := resp["error"].(map[string]any)["message"].(string)
	if errMsg != "registration failed, please try again" {
		t.Fatalf("expected vague error message, got %q", errMsg)
	}
}

func TestAuthSignup_UsernameTooShort(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("user@example.com", "a", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthSignup_UsernameTooLong(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	longUsername := strings.Repeat("a", 33)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", strings.NewReader(
		signupBody("user@example.com", longUsername, "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// ─── authLogin tests ─────────────────────────────────────────────────────────

func TestAuthLogin_Success(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	// 先注册一个用户
	hash, err := auth.HashPassword("Str0ng!Pass")
	if err != nil {
		t.Fatal(err)
	}
	store.CreateUser(context.Background(), "login@example.com", "loginuser", hash)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		loginBody("login@example.com", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Token string         `json:"token"`
		User  map[string]any `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if resp.User["email"] != "login@example.com" {
		t.Fatalf("expected email 'login@example.com', got %v", resp.User["email"])
	}
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	hash, _ := auth.HashPassword("Str0ng!Pass")
	store.CreateUser(context.Background(), "login@example.com", "loginuser", hash)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		loginBody("login@example.com", "Wr0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	msg := resp["error"].(map[string]any)["message"].(string)
	if msg != "invalid email or password" {
		t.Fatalf("expected 'invalid email or password', got %q", msg)
	}
}

func TestAuthLogin_UserNotFound(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		loginBody("nonexistent@example.com", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	msg := resp["error"].(map[string]any)["message"].(string)
	if msg != "invalid email or password" {
		t.Fatalf("expected 'invalid email or password', got %q", msg)
	}
}

func TestAuthLogin_InvalidEmailFormat(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		loginBody("not-an-email", "Str0ng!Pass")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthLogin_EmptyFields(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	tests := []struct {
		name string
		body string
	}{
		{"empty email", loginBody("", "Str0ng!Pass")},
		{"empty password", loginBody("user@example.com", "")},
		{"both empty", loginBody("", "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

// ─── authMe tests ─────────────────────────────────────────────────────────────

func TestAuthMe_ValidToken(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	// 创建用户
	user, _ := store.CreateUser(context.Background(), "me@example.com", "meuser", "fakehash")

	token, err := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["email"] != "me@example.com" {
		t.Fatalf("expected email 'me@example.com', got %v", resp["email"])
	}
	if resp["username"] != "meuser" {
		t.Fatalf("expected username 'meuser', got %v", resp["username"])
	}
}

func TestAuthMe_InvalidToken(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMe_NoToken(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// ─── API Key: Create ─────────────────────────────────────────────────────────

func TestAPIKey_Create(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	// 创建用户
	user, _ := store.CreateUser(context.Background(), "apikey@example.com", "apikeyuser", "fakehash")

	token, _ := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)

	body := `{"name":"my-api-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/api-keys", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Key    string         `json:"key"`
		APIKey map[string]any `json:"api_key"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !strings.HasPrefix(resp.Key, "sk-") {
		t.Fatalf("expected key to start with 'sk-', got %q", resp.Key)
	}
	if resp.APIKey["name"] != "my-api-key" {
		t.Fatalf("expected name 'my-api-key', got %v", resp.APIKey["name"])
	}
}

func TestAPIKey_Create_DefaultName(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	user, _ := store.CreateUser(context.Background(), "apikey@example.com", "apikeyuser", "fakehash")
	token, _ := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/api-keys", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		APIKey map[string]any `json:"api_key"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.APIKey["name"] != "default" {
		t.Fatalf("expected default name 'default', got %v", resp.APIKey["name"])
	}
}

func TestAPIKey_Create_Unauthenticated(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/api-keys", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// ─── API Key: List ───────────────────────────────────────────────────────────

func TestAPIKey_List(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	user, _ := store.CreateUser(context.Background(), "apikey@example.com", "apikeyuser", "fakehash")
	token, _ := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)

	// 先创建一个 API key
	store.CreateAPIKey(context.Background(), user.ID, "sk-abc12345", "hash123", "test-key")

	req := httptest.NewRequest(http.MethodGet, "/api/user/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 api key, got %d", len(resp.Data))
	}
	if resp.Data[0]["name"] != "test-key" {
		t.Fatalf("expected name 'test-key', got %v", resp.Data[0]["name"])
	}
}

func TestAPIKey_List_Unauthenticated(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/user/api-keys", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// ─── API Key: Revoke ─────────────────────────────────────────────────────────

func TestAPIKey_Revoke(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	user, _ := store.CreateUser(context.Background(), "apikey@example.com", "apikeyuser", "fakehash")
	token, _ := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)

	key, _ := store.CreateAPIKey(context.Background(), user.ID, "sk-abc12345", "hash123", "to-revoke")

	req := httptest.NewRequest(http.MethodDelete, "/api/user/api-keys/"+strconv.FormatInt(key.ID, 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %v", resp["status"])
	}

	// 验证 key 已被撤销
	if store.apiKeys[key.ID].Status != "revoked" {
		t.Fatal("expected api key to be revoked")
	}
}

func TestAPIKey_Revoke_NotFound(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	user, _ := store.CreateUser(context.Background(), "apikey@example.com", "apikeyuser", "fakehash")
	token, _ := auth.GenerateToken(user.ID, user.Email, "user", testJWTSecret, 24*time.Hour)

	req := httptest.NewRequest(http.MethodDelete, "/api/user/api-keys/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAPIKey_Revoke_Unauthenticated(t *testing.T) {
	store := newAuthMockStore()
	s := newTestAuthServer(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/user/api-keys/1", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

// ─── validatePasswordStrength unit tests ─────────────────────────────────────

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"strong: 4 categories", "Abcd123!@#", false},
		{"strong: upper+lower+digit", "Abcd1234", false},
		{"strong: upper+lower+special", "Abcdef!!", false},
		{"strong: lower+digit+special", "abcd123!", false},
		{"weak: only digits", "12345678", true},
		{"weak: only lowercase", "abcdefgh", true},
		{"weak: only uppercase", "ABCDEFGH", true},
		{"weak: only special", "!@#$%^&*", true},
		{"weak: lower+digit (2 categories)", "abcd1234", true},
		{"too short", "Ab1!", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := validatePasswordStrength(tt.password)
			if tt.wantErr && msg == "" {
				t.Fatalf("expected error for password %q, got none", tt.password)
			}
			if !tt.wantErr && msg != "" {
				t.Fatalf("expected no error for password %q, got %q", tt.password, msg)
			}
		})
	}
}
