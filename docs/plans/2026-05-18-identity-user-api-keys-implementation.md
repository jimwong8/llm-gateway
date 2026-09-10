# Identity & User API Key MVP Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add user registration, login (JWT), and user-level API Key management to llm-gateway, so end users can self-service obtain API keys and call the OpenAI-compatible API.

**Architecture:**
- New `internal/auth/` package: user domain (password hashing, JWT, API Key generation)
- New `internal/auth/postgres.go`: user and API key persistence
- New `internal/httpserver/auth_handler.go`: REST endpoints for signup/login/API keys
- Modify `internal/httpserver/server.go`: register new routes + JWT middleware
- Modify `web/admin/src/pages/LoginPage.tsx`: add user login/signup forms
- New `web/admin/src/pages/user/ApiKeysPage.tsx`: user API key management UI
- New `web/admin/src/lib/api/identity.ts`: frontend API client
- New `web/admin/src/types/identity.ts`: TypeScript types
- DB migration: `users` table + `user_api_keys` table

**Tech Stack:** Go 1.22+, PostgreSQL, bcrypt/argon2id for passwords, JWT (golang-jwt/jwt v5), React 18, TypeScript

---

## 0. Prerequisites

- Routing Reliability v2 (Chunks 1-8) completed ✅
- Existing `internal/tenant/keys.go` for tenant-level keys (keep as-is)
- New `internal/auth/` package for user-level identity
- DB migration follows existing pattern in `internal/db/migrations/`

## 1. Database Migration

### Task 1: Create migration for users and user_api_keys tables

**Files:**
- Create: `internal/db/migrations/013_users.sql`
- Create: `internal/db/migrations/014_user_api_keys.sql`

- [ ] **Step 1: Write migration SQL**

`013_users.sql`:
```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(64) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role SMALLINT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
```

`014_user_api_keys.sql`:
```sql
CREATE TABLE IF NOT EXISTS user_api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_prefix VARCHAR(8) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    name VARCHAR(128) NOT NULL DEFAULT 'default',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_api_keys_user_id ON user_api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_user_api_keys_prefix ON user_api_keys (key_prefix);
```

- [ ] **Step 2: Verify migration runs**

Check existing migration runner pattern in codebase and ensure new migrations are picked up.

## 2. Auth Domain (internal/auth/)

### Task 2: Password hashing

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestHashPassword(t *testing.T) {
    hash, err := HashPassword("secure-password-123")
    if err != nil {
        t.Fatalf("HashPassword error: %v", err)
    }
    if hash == "" || hash == "secure-password-123" {
        t.Fatal("hash must not be empty or plaintext")
    }
}

func TestVerifyPassword(t *testing.T) {
    hash, _ := HashPassword("secure-password-123")
    if !VerifyPassword(hash, "secure-password-123") {
        t.Fatal("expected password to verify")
    }
    if VerifyPassword(hash, "wrong-password") {
        t.Fatal("expected wrong password to fail")
    }
}
```

- [ ] **Step 2: Run failing tests**

```bash
go test ./internal/auth -run 'TestHashPassword|TestVerifyPassword' -v
```

Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Implement password hashing**

Use bcrypt with cost 12:
```go
func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    return string(hash), err
}

func VerifyPassword(hash, password string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 4: Run tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/password.go internal/auth/password_test.go
git commit -m "feat(auth): add password hashing"
```

### Task 3: JWT generation and validation

**Files:**
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/jwt_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestGenerateAndValidateToken(t *testing.T) {
    secret := "test-secret-key-at-least-32-characters-long"
    token, err := GenerateToken(1, "test@example.com", "user", secret, 24*time.Hour)
    if err != nil {
        t.Fatalf("GenerateToken error: %v", err)
    }
    if token == "" {
        t.Fatal("token must not be empty")
    }

    claims, err := ValidateToken(token, secret)
    if err != nil {
        t.Fatalf("ValidateToken error: %v", err)
    }
    if claims.UserID != 1 {
        t.Fatalf("expected user ID 1, got %d", claims.UserID)
    }
    if claims.Email != "test@example.com" {
        t.Fatalf("expected email test@example.com, got %s", claims.Email)
    }
    if claims.Role != "user" {
        t.Fatalf("expected role user, got %s", claims.Role)
    }
}

func TestValidateToken_InvalidSecret(t *testing.T) {
    token, _ := GenerateToken(1, "test@example.com", "user", "secret-a", 24*time.Hour)
    _, err := ValidateToken(token, "secret-b")
    if err == nil {
        t.Fatal("expected error for wrong secret")
    }
}

func TestValidateToken_Expired(t *testing.T) {
    token, _ := GenerateToken(1, "test@example.com", "user", "test-secret", 1*time.Nanosecond)
    time.Sleep(time.Millisecond)
    _, err := ValidateToken(token, "test-secret")
    if err == nil {
        t.Fatal("expected error for expired token")
    }
}
```

- [ ] **Step 2: Run failing tests**

Expected: FAIL

- [ ] **Step 3: Implement JWT**

Use `golang-jwt/jwt/v5`. If not in go.mod, add it.

```go
type Claims struct {
    UserID int64  `json:"uid"`
    Email  string `json:"email"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func GenerateToken(userID int64, email, role, secret string, ttl time.Duration) (string, error) {
    claims := Claims{
        UserID: userID,
        Email:  email,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    if err != nil {
        return nil, err
    }
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    return nil, errors.New("invalid token")
}
```

- [ ] **Step 4: Run tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/jwt.go internal/auth/jwt_test.go go.mod go.sum
git commit -m "feat(auth): add JWT generation and validation"
```

### Task 4: API Key generation

**Files:**
- Create: `internal/auth/api_key.go`
- Create: `internal/auth/api_key_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestGenerateAPIKey(t *testing.T) {
    key, prefix, hash := GenerateAPIKey()
    if key == "" {
        t.Fatal("key must not be empty")
    }
    if len(prefix) != 8 {
        t.Fatalf("expected prefix length 8, got %d", len(prefix))
    }
    if hash == "" || hash == key {
        t.Fatal("hash must not be empty or equal to plaintext")
    }
    // Key should start with prefix
    if !strings.HasPrefix(key, prefix) {
        t.Fatalf("key %s should start with prefix %s", key, prefix)
    }
}

func TestVerifyAPIKey(t *testing.T) {
    key, _, hash := GenerateAPIKey()
    if !VerifyAPIKey(key, hash) {
        t.Fatal("expected key to verify")
    }
    if VerifyAPIKey("wrong-key", hash) {
        t.Fatal("expected wrong key to fail")
    }
}
```

- [ ] **Step 2: Run failing tests**

Expected: FAIL

- [ ] **Step 3: Implement API key generation**

```go
func GenerateAPIKey() (plaintext, prefix, hash string) {
    raw := make([]byte, 32)
    rand.Read(raw)
    plaintext = "sk-" + hex.EncodeToString(raw)
    prefix = plaintext[:8]
    h, _ := bcrypt.GenerateFromPassword([]byte(plaintext), 10)
    hash = string(h)
    return
}

func VerifyAPIKey(plaintext, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
```

- [ ] **Step 4: Run tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/api_key.go internal/auth/api_key_test.go
git commit -m "feat(auth): add API key generation"
```

### Task 5: User repository

**Files:**
- Create: `internal/auth/postgres.go`
- Create: `internal/auth/postgres_test.go`

- [ ] **Step 1: Write failing tests**

Test user CRUD and API key CRUD with test database or in-memory mock.

- [ ] **Step 2: Implement user repository**

Following existing patterns in `internal/tenant/keys.go` and `internal/admin/postgres.go`.

- [ ] **Step 3: Run tests**

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/auth/postgres.go internal/auth/postgres_test.go
git commit -m "feat(auth): add user and API key repository"
```

## 3. HTTP Handlers

### Task 6: Auth handlers

**Files:**
- Create: `internal/httpserver/auth_handler.go`
- Create: `internal/httpserver/auth_handler_test.go`
- Modify: `internal/httpserver/server.go`

- [ ] **Step 1: Write failing tests**

Test signup, login, and API key CRUD endpoints.

- [ ] **Step 2: Implement auth handlers**

Routes:
- `POST /api/auth/signup` — register new user
- `POST /api/auth/login` — login, return JWT
- `GET /api/auth/me` — get current user info (JWT required)
- `POST /api/user/api-keys` — create API key (JWT required)
- `GET /api/user/api-keys` — list API keys (JWT required)
- `DELETE /api/user/api-keys/{id}` — revoke API key (JWT required)

- [ ] **Step 3: Add JWT middleware**

Modify server.go to add JWT authentication middleware that:
- Extracts bearer token from Authorization header
- Validates JWT
- Attaches user context to request
- Allows admin token to pass through for admin routes

- [ ] **Step 4: Run tests**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpserver/auth_handler.go internal/httpserver/auth_handler_test.go internal/httpserver/server.go
git commit -m "feat(httpserver): add auth handlers and JWT middleware"
```

## 4. Frontend

### Task 7: Identity types and API client

**Files:**
- Create: `web/admin/src/types/identity.ts`
- Create: `web/admin/src/lib/api/identity.ts`

- [ ] **Step 1: Define TypeScript types**

```typescript
export interface User {
  id: number;
  email: string;
  username: string;
  role: number;
  status: string;
  created_at: string;
}

export interface ApiKey {
  id: number;
  name: string;
  key_prefix: string;
  status: string;
  last_used_at?: string;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface SignupRequest {
  email: string;
  username: string;
  password: string;
}

export interface CreateApiKeyRequest {
  name?: string;
}
```

- [ ] **Step 2: Implement API client**

```typescript
import { fetchJson } from './http';

export async function signup(data: SignupRequest): Promise<LoginResponse> {
  return fetchJson('/api/auth/signup', { method: 'POST', body: JSON.stringify(data) });
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  return fetchJson('/api/auth/login', { method: 'POST', body: JSON.stringify(data) });
}

export async function getMe(): Promise<User> {
  return fetchJson('/api/auth/me');
}

export async function listApiKeys(): Promise<ApiKey[]> {
  return fetchJson('/api/user/api-keys');
}

export async function createApiKey(data?: CreateApiKeyRequest): Promise<{ key: string; api_key: ApiKey }> {
  return fetchJson('/api/user/api-keys', { method: 'POST', body: JSON.stringify(data || {}) });
}

export async function revokeApiKey(id: number): Promise<void> {
  return fetchJson(`/api/user/api-keys/${id}`, { method: 'DELETE' });
}
```

- [ ] **Step 3: Commit**

```bash
git add web/admin/src/types/identity.ts web/admin/src/lib/api/identity.ts
git commit -m "feat(frontend): add identity types and API client"
```

### Task 8: Login and signup pages

**Files:**
- Modify: `web/admin/src/pages/LoginPage.tsx`
- Create: `web/admin/src/pages/user/SignupPage.tsx`
- Create: `web/admin/src/pages/user/ApiKeysPage.tsx`
- Modify: `web/admin/src/router.tsx`

- [ ] **Step 1: Refactor LoginPage**

Add tabs for "Admin Login" and "User Login":
- Admin login: existing bearer token flow
- User login: email + password → JWT

- [ ] **Step 2: Create SignupPage**

Simple form: email, username, password, confirm password.

- [ ] **Step 3: Create ApiKeysPage**

List user's API keys, create new key (show plaintext once), revoke keys.

- [ ] **Step 4: Update router**

Add routes:
- `/signup` → SignupPage
- `/user/keys` → ApiKeysPage (protected)

- [ ] **Step 5: Run frontend tests**

```bash
cd web/admin && npm test -- --runInBand
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/admin/src/pages/LoginPage.tsx web/admin/src/pages/user/SignupPage.tsx web/admin/src/pages/user/ApiKeysPage.tsx web/admin/src/router.tsx
git commit -m "feat(frontend): add login/signup/API key pages"
```

## 5. Full Verification

- [ ] **Step 1: Run all backend tests**

```bash
go test ./internal/auth ./internal/httpserver -v
```

Expected: PASS

- [ ] **Step 2: Run frontend tests**

```bash
cd web/admin && npm test -- --runInBand
```

Expected: PASS

- [ ] **Step 3: Build check**

```bash
go build ./...
cd web/admin && npm run build
```

Expected: PASS

- [ ] **Step 4: Commit all remaining**

```bash
git add -A
git commit -m "feat: complete Identity & User API Key MVP"
```

## 6. Acceptance Criteria

- User can sign up with email/username/password
- User can log in and receive JWT
- User can create API keys (plaintext shown once)
- User can list and revoke their API keys
- JWT-authenticated users can call `/v1/chat/completions` with their API key
- Admin token still works for admin routes
- Existing tenant key functionality unchanged
- All tests pass
