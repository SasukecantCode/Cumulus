# Cumulus Backend: Milestone 1 - OAuth Authentication Design

## Executive Summary

Milestone 1 focuses exclusively on Google OAuth 2.0 authentication flow. The user will authenticate, grant Google Drive permissions, receive tokens, and confirm successful authentication via console output. No database, uploads, or storage logic.

This design intentionally structures OAuth to be provider-agnostic, so OAuth adapters for Dropbox, OneDrive, S3, etc. can be swapped in later without rewriting the core authentication orchestration.

---

## 1. OAuth Flow: Step-by-Step

### 1.1 High-Level Flow

```
User Browser                Backend                Google OAuth Server
    |                          |                            |
    | GET /auth/google         |                            |
    |--------(1)-------------->|                            |
    |                          |                            |
    |<--(2)--302 Redirect------|                            |
    |   (to Google)            |                            |
    |                          |                            |
    |                          |    Redirect to /oauth2/auth/google
    |    Google Login & Consent Form                       |
    |--(3) User Signs In & Grants Permission(via Browser)-->|
    |                          |                            |
    |                          |<--(4)--302 Redirect--------|
    |                          |  (auth code + state)       |
    |                          |                            |
    | GET /auth/google/callback|                            |
    | ?code=...&state=...      |                            |
    |--------(5)-------------->|                            |
    |                          |                            |
    |                          | POST /token (5)            |
    |                          | code + client_id + secret  |
    |                          |    (HTTPS only)            |
    |                          |--------(7)---------------->|
    |                          |                            |
    |                          |<--(8)--Tokens------------|
    |                          | access_token              |
    |                          | refresh_token             |
    |                          | id_token (JWT)            |
    |                          |                            |
    |                          | GET /oauth2/v2/userinfo (9)|
    |                          |  (using access_token)     |
    |                          |--------(10)--------------->|
    |                          |                            |
    |                          |<--(11)--User Info-------|
    |                          | email, name, sub (Google ID)
    |                          |                            |
    |<--(12)--Redirect---------|                            |
    | /auth/success (or error) |                            |
    |                          |                            |
```

### 1.2 Detailed Request/Response Breakdown

#### Step 1: User Initiates Login
```
GET /auth/google HTTP/1.1
Host: cumulus-api.example.com

Response: 302 Found
Location: https://accounts.google.com/o/oauth2/v2/auth?
  client_id={CLIENT_ID}&
  redirect_uri=https://cumulus-api.example.com/auth/google/callback&
  response_type=code&
  scope=https://www.googleapis.com/auth/drive&
  state={RANDOM_STATE_VALUE}&
  access_type=offline&
  prompt=consent

Internal: Backend generates random state, stores in ephemeral state cache (in-memory for now).
```

#### Step 2: User Authorizes at Google
User sees Google's consent screen, clicks "Authorize Cumulus to access your Google Drive".

#### Step 3: Google Redirects Back to Callback
```
GET /auth/google/callback?
  code=4/0AY0e-g...(long authorization code)&
  state={SAME_STATE_VALUE}

Internal (Backend):
  1. Extract code and state from query params
  2. Validate state matches stored state (CSRF protection)
  3. Verify code has not expired (5-10 minutes by Google)
  4. Proceed if valid; abort if not
```

#### Step 4: Backend Exchanges Code for Tokens
```
POST https://oauth2.googleapis.com/token HTTP/1.1
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
code={AUTH_CODE}&
client_id={CLIENT_ID}&
client_secret={CLIENT_SECRET}&
redirect_uri=https://cumulus-api.example.com/auth/google/callback

Response:
{
  "access_token": "ya29.a0AfH6SMB...",
  "expires_in": 3599,
  "refresh_token": "1//0gkK...",
  "scope": "https://www.googleapis.com/auth/drive",
  "token_type": "Bearer",
  "id_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6IjI8auth jwt>..."
}

Internal:
  - access_token: Valid for ~1 hour, used for API calls
  - refresh_token: Long-lived (no expiry unless revoked), used to get new access tokens
  - id_token: JWT containing user info (email, sub), can be decoded locally
  - expires_in: Token TTL in seconds
```

#### Step 5: Backend Fetches User Info (Optional but Recommended)
```
GET https://www.googleapis.com/oauth2/v2/userinfo HTTP/1.1
Authorization: Bearer {ACCESS_TOKEN}

Response:
{
  "id": "118364973454...",        // Google Account ID (sub)
  "email": "user@gmail.com",
  "verified_email": true,
  "name": "John Doe",
  "picture": "https://..."
}
```

**Question: ID Token vs UserInfo?**
- **ID Token (JWT):** Can be decoded locally without a network call. Contains `sub` (Google ID), `email`, `email_verified`.
- **UserInfo Endpoint:** Requires network call but is the authoritative source.

**For Milestone 1 Recommendation:** Use the ID Token (from JWT decode) because:
1. No additional network round-trip
2. Contains sufficient data (email + Google ID)
3. Reduces complexity for MVP
4. Later milestones can fetch full profile from UserInfo if needed

---

## 2. Layer Responsibilities

### 2.1 Transport Layer (HTTP Handlers)

**File:** `internal/transport/http/handlers/auth_handler.go`

Responsibilities:
- Parse HTTP requests (query params, form data)
- Extract authorization codes and state values
- Delegate to service layer
- Format HTTP responses (redirects, JSON, status codes)
- Return appropriate HTTP status codes and error messages to client

What it **does NOT** do:
- Generate OAuth URLs
- Exchange tokens
- Validate OAuth responses
- Call Google APIs

Example flow:
```go
Handler receives GET /auth/google
  → Call AuthService.StartGoogleLogin()
  → Get OAuth URL
  → Return 302 redirect
```

### 2.2 Service Layer (Business Logic)

**File:** `internal/usecase/auth_service.go`

Responsibilities:
- Orchestrate OAuth workflow
- Generate state values for CSRF protection
- Call infrastructure layer to exchange codes
- Validate OAuth responses
- Apply business rules (logging, tracking auth attempts)
- Prepare data for future persistence

Does NOT do:
- Persist data to database
- Make HTTP requests directly
- Copy OAuth logic across providers

Example flow:
```go
AuthService.StartGoogleLogin(ctx)
  → Generate random state
  → Call OAuthConfigProvider.GetAuthURL(state)
  → Return OAuth login URL

AuthService.HandleCallback(ctx, code, state)
  → Validate state (CSRF check)
  → Call OAuthProvider.ExchangeCode(code)
  → Get tokens
  → Call OAuthProvider.GetUserInfo()
  → Return authenticated user info
```

### 2.3 Infrastructure Layer (External Integrations)

**File:** `internal/infrastructure/google/oauth.go`

Responsibilities:
- Implement OAuth protocol specifics (Google)
- Make HTTP calls to Google APIs
- Handle token exchange
- Fetch user information from Google
- Parse and validate Google responses

Does NOT do:
- Apply business logic
- Make decisions about authentication
- Persist credentials

Example:
```go
GoogleOAuthProvider.ExchangeCode(ctx, code, state)
  → POST to Google token endpoint
  → Parse token response
  → Return OAuthTokens struct

GoogleOAuthProvider.GetUserInfo(ctx, accessToken)
  → GET /oauth2/v2/userinfo
  → Parse response
  → Return user email and Google ID
```

### 2.4 Configuration Layer

**File:** `internal/config/config.go`

Responsibilities:
- Load OAuth credentials (Client ID, Client Secret, Redirect URL)
- Provide configuration to infrastructure layer
- Validate that required secrets are present

Does NOT do:
- Hard-code secrets
- Make assumptions about where secrets come from

### 2.5 Repository Layer (Persistence - Deferred)

For Milestone 1: **SKIPPED**. No database queries.

For Milestone 2+:
- Save authenticated accounts to PostgreSQL
- Retrieve accounts by user ID
- Update token refresh status

---

## 3. Project Structure for Milestone 1

```
cumulus/
├── cmd/
│   └── cumulus/
│       └── main.go                          # Application entrypoint
│
├── internal/
│   ├── config/
│   │   └── config.go                        # Config struct, environment loading
│   │                                        # Loads: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
│   │                                        #        GOOGLE_REDIRECT_URL, GOOGLE_SCOPES
│   │
│   ├── domain/
│   │   └── ports.go                         # Interfaces for OAuthProvider, OAuthTokens
│   │                                        # (Keep database models out for now)
│   │
│   ├── usecase/
│   │   └── auth_service.go                  # AuthService: orchestrates OAuth flow
│   │                                        # Methods:
│   │                                        #   - StartGoogleLogin(ctx) -> string (OAuth URL)
│   │                                        #   - HandleCallback(ctx, code, state) -> AuthResult
│   │                                        #   - Validates state, calls OAuth provider
│   │
│   ├── infrastructure/
│   │   └── google/
│   │       ├── oauth.go                     # GoogleOAuthProvider: implements OAuthProvider
│   │       │                                # Methods:
│   │       │                                #   - ExchangeCode(ctx, code, state)
│   │       │                                #   - GetUserInfo(ctx, accessToken)
│   │       │                                #   - GetAuthURL(state) -> string
│   │       │
│   │       └── scopes.go                    # Constants for Google scopes
│   │                                        # SCOPE_DRIVE_READONLY, SCOPE_DRIVE, etc.
│   │
│   ├── transport/
│   │   └── http/
│   │       ├── handlers/
│   │       │   └── auth_handler.go          # HTTP handlers
│   │       │                                # Handlers:
│   │       │                                #   - GET /auth/google
│   │       │                                #   - GET /auth/google/callback
│   │       │
│   │       ├── middleware/
│   │       │   └── cors.go                  # CORS middleware (if needed)
│   │       │
│   │       └── routes.go                    # Route registration
│   │
│   └── utils/
│       └── state.go                         # State generation and validation
│                                            # Functions:
│                                            #   - GenerateState() -> string
│                                            #   - ValidateState(stored, received) -> bool
│                                            #   - StateCache interface (in-memory for now)
│
├── go.mod
├── go.sum
└── docs/
    ├── architecture.md                      # (existing)
    └── milestone_1_design.md                # (this document)
```

### File Responsibilities Summary

| File | Responsibility |
|------|-----------------|
| `main.go` | Bootstrap Gin engine, load config, register routes, start server |
| `config/config.go` | Load env vars, validate OAuth secrets exist |
| `domain/ports.go` | Define OAuthProvider interface, OAuthTokens struct |
| `usecase/auth_service.go` | Orchestrate OAuth flow, validate state, manage workflow |
| `infrastructure/google/oauth.go` | Implement Google OAuth protocol, token exchange |
| `infrastructure/google/scopes.go` | Google scope constants |
| `transport/http/handlers/auth_handler.go` | Parse requests, call AuthService, return responses |
| `transport/http/routes.go` | Register /auth/google and /auth/google/callback routes |
| `utils/state.go` | Generate, store, and validate CSRF state tokens |

---

## 4. Configuration Strategy

### 4.1 Environment Variables

For Milestone 1, use environment variables (loaded via `os.Getenv` or Viper):

```bash
# Required
GOOGLE_CLIENT_ID=<from Google Console>
GOOGLE_CLIENT_SECRET=<from Google Console>
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Optional with sensible defaults
HTTP_PORT=8080
GOOGLE_SCOPES=https://www.googleapis.com/auth/drive
LOG_LEVEL=info
```

### 4.2 Configuration Loading

**Approach: Use Viper for centralized config management**

Why Viper?
- Supports env vars, `.env` files, YAML, JSON
- Can override via flags
- Structured config validation
- Easier to test (inject mock config)

```go
// internal/config/config.go

type Config struct {
    HTTPPort     string
    GoogleOAuth  GoogleOAuthConfig
    LogLevel     string
}

type GoogleOAuthConfig struct {
    ClientID    string
    ClientSecret string
    RedirectURL string
    Scopes      []string
}

func Load() (*Config, error) {
    viper.SetDefault("HTTP_PORT", "8080")
    viper.SetDefault("LOG_LEVEL", "info")
    
    viper.BindEnv("GOOGLE_CLIENT_ID")
    viper.BindEnv("GOOGLE_CLIENT_SECRET")
    viper.BindEnv("GOOGLE_REDIRECT_URL")
    
    // Validate required fields
    if viper.GetString("GOOGLE_CLIENT_ID") == "" {
        return nil, errors.New("GOOGLE_CLIENT_ID not set")
    }
    
    return &Config{...}, nil
}
```

### 4.3 Secrets Management

**For Milestone 1 (Development):**
- Use `.env` file (add to `.gitignore`)
- Load via `godotenv` or Viper

**For Milestone 2+ (Production):**
- Cloud secret manager (AWS Secrets Manager, Google Secret Manager, HashiCorp Vault)
- Rotate secrets regularly
- Never commit secrets to git

### 4.4 Why NOT Hard-Code?

Hard-coding means:
- Configuration changes require code rebuilds
- Secrets visible in git history
- Difficult to run multiple environments (dev, staging, prod)
- Breaking change to add new OAuth providers

Env vars allow:
- Configuration without code changes
- Easy environment switching
- Integration with secret management systems
- No code rebuild needed

---

## 5. OAuth State Management & CSRF Protection

### 5.1 The State Parameter Problem

Without state protection:
1. User visits `/auth/google`
2. Backend generates OAuth URL, redirects user to Google
3. User grants permission
4. Google redirects to callback with auth code
5. **ATTACKER:** Meanwhile, attacker crafted their own OAuth URL, tricks user into visiting it
6. User is redirected to Google, but attacker's redirect URI is called
7. Attacker now has access token for user's Google account

### 5.2 State Protection Solution

1. **Backend generates random state:** `state = generateSecureRandomString(32)`
2. **Backend stores state temporarily:** In-memory cache, Redis, or database
3. **State included in OAuth URL:** Sent to Google
4. **Google includes state in callback:** Returns `?code=...&state=<same value>`
5. **Backend validates state matches:** Before accepting authorization code

### 5.3 State Implementation

```go
// internal/utils/state.go

type StateManager interface {
    GenerateState() string
    ValidateState(state string) bool
}

// In-memory implementation for Milestone 1
type InMemoryStateManager struct {
    mu     sync.Mutex
    states map[string]time.Time  // state -> expiration time
}

func (m *InMemoryStateManager) GenerateState() string {
    state := generateSecureRandom(32)  // crypto/rand
    m.mu.Lock()
    m.states[state] = time.Now().Add(10 * time.Minute)
    m.mu.Unlock()
    return state
}

func (m *InMemoryStateManager) ValidateState(state string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    expiry, exists := m.states[state]
    if !exists || time.Now().After(expiry) {
        return false
    }
    
    delete(m.states, state)  // One-time use
    return true
}
```

### 5.4 State Lifecycle

- **Generated:** When user accesses `/auth/google`
- **Stored:** In-memory with 10-minute TTL
- **Validated:** When callback arrives
- **Invalidated:** After validation (one-time use)
- **Cleaned Up:** Expired states removed periodically

---

## 6. Google Scopes

### 6.1 Required Scopes for Milestone 1

For Google Drive access:

| Scope | Purpose | Required for M1? |
|-------|---------|------------------|
| `https://www.googleapis.com/auth/drive` | Full access to user's Drive files | **YES** (needed for future uploads) |
| `https://www.googleapis.com/auth/drive.readonly` | Read-only access | NO (we need write access) |
| `https://www.googleapis.com/auth/drive.metadata.readonly` | Read file metadata only | NO (too restrictive) |
| `openid` | Request ID Token | YES (contains user info) |
| `email` | Request email claim | YES (identifies user) |
| `profile` | Request profile fields (name, picture) | NO (nice-to-have for future) |

### 6.2 Recommended Scope String for M1

```go
// internal/infrastructure/google/scopes.go

const (
    ScopeDrive = "https://www.googleapis.com/auth/drive"
    ScopeOpenID = "openid"
    ScopeEmail = "email"
)

// For Milestone 1, combine into single string
var M1Scopes = []string{
    ScopeDrive,     // Future uploads
    ScopeOpenID,    // ID Token
    ScopeEmail,     // Email validation
}

// In OAuth URL:
// scope=https://www.googleapis.com/auth/drive openid email
// (space-separated)
```

### 6.3 Why Not Request More?

- Users see longer permission screen (scary)
- Violates principle of least privilege
- Increases blast radius if credentials compromised
- Easy to add more scopes later

---

## 7. Callback Processing & Error Handling

### 7.1 Callback URL Structure

```
GET /auth/google/callback?code=...&state=...

Query Parameters:
  code:  Authorization code (expires in ~10 minutes)
  state: CSRF protection value
  
Possible Error Parameters:
  error: error code (access_denied, invalid_scope, etc.)
  error_description: human-readable error
  error_uri: link to error documentation
```

### 7.2 Callback Processing Flow

```go
HandleCallback(code, state string) {
    // Step 1: CSRF Validation
    if !stateManager.ValidateState(state) {
        return Error("Invalid or expired state", 400)
    }
    
    // Step 2: Code Exchange
    tokens, err := googleOAuth.ExchangeCode(code)
    if err != nil {
        return handleTokenExchangeError(err)
    }
    
    // Step 3: User Info Retrieval
    userInfo, err := googleOAuth.GetUserInfo(tokens.AccessToken)
    if err != nil {
        return handleUserInfoError(err)
    }
    
    // Step 4: Validation
    if !isValidEmail(userInfo.Email) {
        return Error("Invalid email domain", 400)
    }
    
    // Step 5: Success
    return DisplaySuccess(userInfo, tokens)
}
```

### 7.3 Error Handling Scenarios

#### User Cancels Permission

**What Happens:**
```
Google redirects to:
GET /auth/google/callback?error=access_denied&error_description=The+user+denied+access
```

**Backend Response:**
```
Status: 400 Bad Request
Body: "Authorization denied by user"
```

#### State Mismatch

**Cause:** CSRF attack or session expired

**Backend Response:**
```
Status: 400 Bad Request
Body: "Invalid or expired state. Please try again."
```

#### Authorization Code Expired

**Cause:** User took >10 minutes between OAuth redirect and callback

**Backend Response:**
```
Status: 400 Bad Request
Body: "Authorization code expired. Please log in again."
```

#### Invalid Authorization Code

**Cause:** Malformed code, already used, or tampered

**Backend Response:**
```
Status: 401 Unauthorized
Body: "Invalid authorization code. Please log in again."
```

#### Google API Error (Rate Limit, Server Error, etc.)

**Cause:** Google services unavailable, rate limited, or other server error

**Backend Response:**
```
Status: 503 Service Unavailable
Body: "Google services temporarily unavailable. Please try again later."
```

#### Network Failure

**Cause:** Connection timeout to Google

**Backend Response:**
```
Status: 503 Service Unavailable
Body: "Connection to Google services failed. Please try again."
With Retry-After header
```

#### Invalid Access Token

**Cause:** Token was revoked or tampered

**Backend Response:**
```
Status: 401 Unauthorized
Body: "Failed to verify your Google account. Please log in again."
```

### 7.4 Error Response Structure

For Milestone 1, console output is sufficient. But design for consistency:

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Code    string `json:"code,omitempty"`
}

// Examples:
ErrorResponse{
    Error:   "access_denied",
    Message: "User declined authorization",
    Code:    "GOOGLE_ACCESS_DENIED",
}

ErrorResponse{
    Error:   "state_mismatch",
    Message: "Invalid or expired state token",
    Code:    "STATE_VALIDATION_FAILED",
}
```

---

## 8. Token Handling Explanation

### 8.1 Token Types & Lifespans

| Token | Lifespan | Scope | Use Case |
|-------|----------|-------|----------|
| **Authorization Code** | ~10 minutes | One-time use | Exchange for tokens |
| **Access Token** | ~1 hour | API calls | Call Google APIs |
| **Refresh Token** | No expiry (unless revoked) | Token refresh | Get new access token |
| **ID Token** | ~1 hour | User identity | Contains email, sub (locally validated) |

### 8.2 Token Lifecycle

```
User Browser              Backend                    Google
    |                        |                         |
    | /auth/google           |                         |
    |------(1)------->       |                         |
    |                        |                         |
    |<--(2) Redirect---------|                         |
    |  to Google OAuth       |                         |
    |                        |                         |
    | Google Login Flow      |---(3) Redirect Auth Code|
    |<----(4) Code + State---|                         |
    |                        |                         |
    | /callback?code=...     |                         |
    |------(5)------->       |                         |
    |                        | POST /token             |
    |                        | code + client_secret   |
    |                        |-----(6)---------------->|
    |                        |                         |
    |                        |<--(7) Tokens-----------|
    |                        | access_token (1 hr)    |
    |                        | refresh_token (no exp) |
    |                        | id_token (JWT, 1 hr)   |
    |                        |                         |
    |                        | GET /userinfo          |
    |                        |  with access_token    |
    |                        |-----(8)---------------->|
    |                        |                         |
    |                        |<--(9) User Info-------|
    |                        |                         |
    |<--(10) Success---------|                         |
    | { "email": "...",      |                         |
    |   "google_id": "...",  |                         |
    |   "access_token": "...",                         |
```

### 8.3 Storage Strategy

For Milestone 1: **DO NOT STORE** (console output only)

For Milestone 2+:
- **Refresh Token:** Store in PostgreSQL (will be needed to refresh access tokens later)
- **Access Token:** DO NOT store long-term (refresh from Google instead)
- **ID Token:** DO NOT store (stateless validation only)

Why?
- If access token leaked, attacker has limited time (1 hour) to abuse it
- Refresh token needed once per month to keep user logged in
- ID Token is stateless; can be re-requested if needed

### 8.4 Access Token Refresh Flow (Future)

Once stored, refresh tokens enable long-term access:

```go
// When access token expires (future milestone)
POST https://oauth2.googleapis.com/token
grant_type=refresh_token&
refresh_token={STORED_REFRESH_TOKEN}&
client_id={CLIENT_ID}&
client_secret={CLIENT_SECRET}

// Response:
{
  "access_token": "ya29.new_token...",
  "expires_in": 3599,
  "token_type": "Bearer"
}
```

---

## 9. User Identity Retrieval Strategy

### 9.1 ID Token vs UserInfo Endpoint

#### Option A: ID Token (Recommended for M1)

**Process:**
1. Google returns `id_token` in token exchange response
2. Backend decodes JWT locally (validate signature using Google's public keys)
3. Extract claims: `sub` (Google ID), `email`, `email_verified`

**Advantages:**
- No network call (fast)
- Self-contained (no server dependency)
- Cryptographically signed
- Contains essential user info

**Disadvantages:**
- Requires JWT library and key validation
- Missing profile data (name, picture)

#### Option B: UserInfo Endpoint

**Process:**
1. Use access token to call `/oauth2/v2/userinfo`
2. Google returns user profile

**Advantages:**
- Simple (no JWT complexity)
- Contains full profile (name, picture, locale, etc.)

**Disadvantages:**
- Additional network round-trip
- Requires valid access token
- Slower

### 9.2 Recommendation for Milestone 1

**Use ID Token (Option A)** because:
1. Milestone 1 only needs email + Google ID
2. One fewer network call (faster)
3. ID Token is designed for this use case
4. Simpler error handling

```go
// Milestone 1 Implementation
type UserInfo struct {
    GoogleID  string  // sub claim
    Email     string  // email claim
    Verified  bool    // email_verified claim
}

// Decode ID Token (JWT)
claims, err := jwt.ParseWithClaims(idToken, &jwt.StandardClaims{}, keyFunc)
if err != nil {
    return nil, err
}

userInfo := UserInfo{
    GoogleID: claims.Subject,
    Email:    claims.Email,
    Verified: claims.EmailVerified,
}
```

**Optional for Future:** Store UserInfo endpoint call for profile enrichment (name, picture) in Milestone 2+.

---

## 10. Future Extensibility: Multi-Provider Architecture

### 10.1 The Problem with Provider-Specific Code

Without proper abstraction, adding Dropbox looks like:

```go
// BAD: Couple business logic to provider
if provider == "google" {
    tokens = googleOAuth.ExchangeCode(code)
    userInfo = googleOAuth.GetUserInfo(tokens)
} else if provider == "dropbox" {
    tokens = dropboxOAuth.ExchangeCode(code)
    userInfo = dropboxOAuth.GetUserInfo(tokens)
} else if provider == "onedrive" {
    ...
}
```

Problems:
- Duplicated logic
- Hard to test
- Difficult to add new providers
- Business logic scattered across providers

### 10.2 Solution: Provider Abstraction

Define a **Provider Interface** that each cloud service implements:

```go
// internal/domain/ports.go

type OAuthProvider interface {
    // Get OAuth login URL
    GetAuthURL(state string) string
    
    // Exchange authorization code for tokens
    ExchangeCode(ctx context.Context, code string) (*OAuthTokens, error)
    
    // Get user identity
    GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
    
    // Get provider name and type
    ProviderName() string
    ProviderType() string  // "google", "dropbox", "onedrive", etc.
}

type OAuthTokens struct {
    AccessToken  string
    RefreshToken string
    ExpiryUnix   int64
}

type UserInfo struct {
    ProviderID string  // sub/id from provider
    Email      string
    Name       string
    Picture    string
}
```

### 10.3 Provider Implementations

```
internal/infrastructure/
├── google/
│   ├── oauth.go          // implements OAuthProvider
│   └── scopes.go
├── dropbox/              // (future)
│   ├── oauth.go
│   └── scopes.go
├── onedrive/             // (future)
│   ├── oauth.go
│   └── scopes.go
└── s3/                   // (future)
    ├── iam.go
    └── ...
```

### 10.4 Business Logic Becomes Provider-Agnostic

```go
// internal/usecase/auth_service.go

type AuthService struct {
    stateManager StateManager
    providers    map[string]OAuthProvider  // "google" -> GoogleProvider
                                           // "dropbox" -> DropboxProvider
}

func (s *AuthService) StartLogin(providerName string, state string) string {
    provider := s.providers[providerName]
    return provider.GetAuthURL(state)
}

func (s *AuthService) HandleCallback(providerName string, code string, state string) (*UserInfo, error) {
    provider := s.providers[providerName]
    
    if !s.stateManager.ValidateState(state) {
        return nil, errors.New("invalid state")
    }
    
    tokens, err := provider.ExchangeCode(ctx, code)
    if err != nil {
        return nil, err
    }
    
    return provider.GetUserInfo(ctx, tokens.AccessToken)
}
```

### 10.5 Routes Become Provider-Parameterized

```go
// internal/transport/http/routes.go

func RegisterAuthRoutes(router *gin.Engine, authService *AuthService) {
    // Supports any provider
    router.GET("/auth/:provider", handlers.StartLogin(authService))
    router.GET("/auth/:provider/callback", handlers.Callback(authService))
}

// Usage:
// GET /auth/google → Start Google OAuth
// GET /auth/dropbox → Start Dropbox OAuth
// GET /auth/:provider/callback?code=...&state=... → Handle callback
```

### 10.6 Extensibility Checklist for Milestone 1

- [ ] Define `OAuthProvider` interface in `domain/ports.go`
- [ ] Implement `GoogleOAuthProvider` in `infrastructure/google/`
- [ ] Use provider via interface (not concrete type)
- [ ] AuthService accepts provider as dependency (not hard-coded)
- [ ] Routes parameterized by provider name
- [ ] No OAuth provider logic in handlers or services

---

## 11. Testing Strategy

### 11.1 Manual Testing (Milestone 1)

Since no database, testing is integration-style with real Google OAuth.

#### Test Scenario 1: Successful Authentication

```
1. Start server: go run cmd/cumulus/main.go
2. Browser: GET http://localhost:8080/auth/google
3. Backend redirects to Google OAuth consent
4. User signs in with Google account
5. User clicks "Allow" button
6. Google redirects to callback with code
7. Backend exchanges code for tokens
8. Backend displays:
   "Authenticated successfully
    Email: user@gmail.com
    Google Account ID: 118364973454...
    Scopes: "openid email https://www.googleapis.com/auth/drive"
    Refresh Token exists: yes"
```

**Success Criteria:**
- No errors in console
- All user info printed correctly
- Tokens received from Google

#### Test Scenario 2: User Cancels

```
1. Start server
2. Browser: GET http://localhost:8080/auth/google
3. Google redirect shown
4. User clicks "Cancel" at Google consent
5. Google redirects with error=access_denied
6. Backend displays:
   "Authorization failed: User denied access"
```

**Success Criteria:**
- Error gracefully handled
- No exceptions/stack traces in logs
- Clear error message

#### Test Scenario 3: State Mismatch (CSRF Attack Simulation)

```
1. Generate two different state values: state1, state2
2. Start OAuth with state1
3. Manually tamper callback to use state2
4. Backend returns:
   "Invalid or expired state"
```

**Success Criteria:**
- State validation works
- Tampered request rejected

#### Test Scenario 4: Expired State

```
1. Start OAuth, get state value
2. Wait 11+ minutes
3. Manually call callback with original state
4. Backend returns:
   "Invalid or expired state"
```

**Success Criteria:**
- State expires after configured TTL
- Expired state rejected

#### Test Scenario 5: Invalid Authorization Code

```
1. Start OAuth
2. Manually call callback with invalid code: "invalid_code_xyz"
3. Backend returns:
   "Invalid authorization code"
```

**Success Criteria:**
- Google rejects invalid code
- Backend handles gracefully

### 11.2 Automated Testing (Future)

For Milestone 2+, add unit tests using mocks:

```go
// Test: AuthService validates state
type MockStateManager struct {
    ValidateCalled bool
}

func (m *MockStateManager) ValidateState(state string) bool {
    m.ValidateCalled = true
    return state == "valid_state"
}

// Test: Service calls OAuth provider
type MockOAuthProvider struct {
    ExchangeCodeCalled bool
}

func (m *MockOAuthProvider) ExchangeCode(ctx context.Context, code string) (*OAuthTokens, error) {
    m.ExchangeCodeCalled = true
    return &OAuthTokens{...}, nil
}
```

### 11.3 Observable Behavior (Logging)

For testability without database, log each step:

```
[INFO] Auth flow starting for provider: google
[INFO] Generated state: abc123xyz... (expires in 10m)
[INFO] Redirecting to Google OAuth
[INFO] Callback received with code and state
[INFO] State validation: PASSED
[INFO] Exchanging code for tokens...
[INFO] Token exchange successful
[INFO] Fetching user info...
[INFO] User info retrieved: email=user@gmail.com
[INFO] Authentication successful
[INFO] Email: user@gmail.com
[INFO] Google ID: 118364973454
[INFO] Refresh token exists: yes
[INFO] Scopes: openid email https://www.googleapis.com/auth/drive
```

---

## 12. Security Considerations

### 12.1 HTTPS Requirement

Feature | HTTPS | HTTP |
|---------|-------|------|
| Token Exchange | **REQUIRED** | ⚠️ Credentials exposed |
| Callbacks | **REQUIRED** | ⚠️ Code/tokens visible |
| OAuth URLs | **REQUIRED** | ⚠️ State/code intercepted |

**Enforce:** Disable HTTP in production. Use HTTPS-only redirect.

### 12.2 State Token Security

```go
// DO: Use cryptographically secure random
import "crypto/rand"

func GenerateState() string {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        panic(err)  // Safe to panic: crypto is critical
    }
    return hex.EncodeToString(b)
}

// DON'T: Use math/rand (predictable)
// DON'T: Use UUID (insufficient entropy for CSRF)
```

### 12.3 Credential Handling

- **Client Secret:** Never log, never commit
- **Access Token:** Short-lived (1 hour), refreshed via refresh token
- **Refresh Token:** Stored encrypted in database (future)
- **ID Token:** Validated locally, never sent to external services

### 12.4 Code Exchange Must Be Server-to-Server

```
// CORRECT: Backend exchanges code server-to-server
Backend POST to Google with client_secret

// WRONG: Frontend exchanges code
Browser sends code + client_secret to Google (exposes secret!)
```

### 12.5 Redirect URL Validation

```go
// CORRECT: Exact match validation
googleConfig := &oauth2.Config{
    RedirectURL: "https://cumulus-api.example.com/auth/google/callback",
    // ...
}

// File: internal/transport/http/handlers/auth_handler.go
expectedRedirectURL := "https://cumulus-api.example.com/auth/google/callback"
if config.RedirectURL != expectedRedirectURL {
    return Error("Redirect URL mismatch", 400)
}
```

### 12.6 Scope Validation

```go
// Verify Google returned expected scopes
returnedScopes := tokens.Scope  // "openid email drive"
expectedScopes := []string{"openid", "email", "drive"}

if !hasScopesMulti(returnedScopes, expectedScopes) {
    return Error("Google did not grant required scopes", 403)
}
```

### 12.7 Rate Limiting

Add rate limiting to prevent brute force / DoS:

```go
// Example: Limit to 10 login attempts per IP per minute
router.POST("/auth/google", rateLimiter.Limit(10, time.Minute), startLoginHandler)
```

---

## 13. Configuration Strategy Summary

### 13.1 Deployment Configurations

#### Local Development
```bash
export HTTP_PORT=8080
export GOOGLE_CLIENT_ID=<dev-client-id>
export GOOGLE_CLIENT_SECRET=<dev-client-secret>
export GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
```

#### Staging
```bash
export HTTP_PORT=8080
export GOOGLE_CLIENT_ID=<staging-client-id>
export GOOGLE_CLIENT_SECRET=<staging-secret>
export GOOGLE_REDIRECT_URL=https://staging-cumulus.example.com/auth/google/callback
```

#### Production
```bash
export HTTP_PORT=8080
export GOOGLE_CLIENT_ID=<prod-client-id>
export GOOGLE_CLIENT_SECRET=<from-secret-manager>
export GOOGLE_REDIRECT_URL=https://api.cumulus.example.com/auth/google/callback
```

### 13.2 Configuration File (Recommended for Production)

```yaml
# config.yaml
http:
  port: 8080
  
logging:
  level: info

oauth:
  google:
    client_id: ${GOOGLE_CLIENT_ID}  # From env or secret manager
    client_secret: ${GOOGLE_CLIENT_SECRET}
    redirect_url: ${GOOGLE_REDIRECT_URL}
    scopes:
      - openid
      - email
      - https://www.googleapis.com/auth/drive
    state_ttl_minutes: 10
```

### 13.3 Viper Configuration Load

```go
// internal/config/config.go
func Load() (*Config, error) {
    viper.AddConfigPath("./config")
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    
    // Bind env vars (override config file)
    viper.BindEnv("google.client_id", "GOOGLE_CLIENT_ID")
    viper.BindEnv("google.client_secret", "GOOGLE_CLIENT_SECRET")
    viper.BindEnv("google.redirect_url", "GOOGLE_REDIRECT_URL")
    
    if err := viper.ReadInConfig(); err != nil {
        // If config file doesn't exist, use env vars
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
        }
    }
    
    // Validate required config exists
    if viper.GetString("google.client_id") == "" {
        return nil, errors.New("GOOGLE_CLIENT_ID not set")
    }
    
    return &Config{...}, nil
}
```

---

## Architecture Challenges & Proposed Improvements

### Challenge 1: Stateful State Management

**Current Design:** In-memory state cache with TTL.

**Issue:** Doesn't scale to multiple instances. If user's callback hits different server than the one that issued state, validation fails.

**Proposed Solution:** 
- For MVP: Accept single-instance limitation, use in-memory
- For Scale: Move to Redis or distributed cache
- Code design: Define `StateManager` interface so swap is easy

**Decision:** Proceed with in-memory for M1, document need for Redis before going multi-instance.

```go
type StateManager interface {
    GenerateState() string
    ValidateState(state string) bool
}

// Milestone 1: In-memory
type InMemoryStateManager struct { ... }

// Milestone 2+: Redis
type RedisStateManager struct { ... }
```

---

### Challenge 2: Refresh Token Persistence

**Current Design:** No persistence in Milestone 1.

**Issue:** After user logs in, we lose the refresh token. Can't refresh access when it expires.

**Proposed Solution:**
- M1: Accept console-only output (no re-authentication)
- M2: Save refresh token to PostgreSQL
- M3: Implement periodic refresh job

**Design today to enable this:**
- AuthService receives UserRepository dependency (but doesn't use it in M1)
- AuthService ready to call `userRepository.Save()` in M2
- This avoids refactoring AuthService later

```go
// Milestone 1: AuthService has repo but doesn't use
type AuthService struct {
    oauthProvider OAuthProvider
    stateManager  StateManager
    userRepo      UserRepository  // Injected but unused in M1
}

// Milestone 2: Same AuthService calls userRepo.Save()
func (s *AuthService) HandleCallback(...) (*UserInfo, error) {
    // ... token exchange ...
    
    // NEW: Save user (M2+)
    if s.userRepo != nil {
        s.userRepo.Create(ctx, &User{
            Email:        userInfo.Email,
            GoogleID:     userInfo.GoogleID,
            RefreshToken: tokens.RefreshToken,
        })
    }
    
    return userInfo, nil
}
```

---

### Challenge 3: Error Recovery & Retry Logic

**Current Design:** Errors returned immediately.

**Issue:** Network hiccup loses the flow. User must restart.

**Proposed Solution:**
- M1: No retry logic (acceptable for MVP)
- M2: Implement exponential backoff for Google API calls
- Design: OAuthProvider methods return `RetryableError` type

```go
type RetryableError struct {
    Err       error
    Retryable bool
    After     time.Duration
}

// Usage:
tokens, err := provider.ExchangeCode(ctx, code)
if err != nil {
    if retErr, ok := err.(*RetryableError); ok && retErr.Retryable {
        // Retry after duration
        time.Sleep(retErr.After)
        tokens, _ = provider.ExchangeCode(ctx, code)
    }
}
```

---

### Challenge 4: Testing Without Real Google Credentials

**Current Design:** Manual testing with real Google OAuth app.

**Issue:** Hard to test in CI, risk of exposing credentials.

**Proposed Solution:**
- M1: Manual testing only (OAuth flow too complex to mock)
- M2: Create OAuthProvider mock implementation
- Design: Interface in domain means mocking is possible

```go
type MockGoogleOAuthProvider struct {
    tokens   OAuthTokens
    userInfo UserInfo
}

func (m *MockGoogleOAuthProvider) ExchangeCode(ctx context.Context, code string) (*OAuthTokens, error) {
    return &m.tokens, nil
}
```

---

### Challenge 5: Provider Registration / Dependency Injection

**Current Design:** Maps providers to names.

**Issue:** How to register providers in a scalable way?

**Proposed Solution:**
- Use dependency injection container
- Register providers in `main.go`
- AuthService receives provider map

```go
// main.go
func main() {
    googleProvider := &GoogleOAuthProvider{...}
    
    providers := map[string]OAuthProvider{
        "google": googleProvider,
    }
    
    authService := usecase.NewAuthService(
        stateManager,
        providers,
    )
    
    // Later: Add dropbox, onedrive, etc.
    // providers["dropbox"] = &DropboxOAuthProvider{...}
}
```

---

### Challenge 6: Session Management Missing

**Current Design:** No session management.

**Issue:** How do we know who the current user is after login?

**Proposed Solution:**
- M1: Not needed (console output only)
- M2: Issue JWT or session cookie
- Design: Add SessionManager interface

```go
type SessionManager interface {
    CreateSession(ctx context.Context, userID string) (sessionToken string, error)
    ValidateSession(ctx context.Context, token string) (userID string, error)
}

// Milestone 1: Skipped
// Milestone 2: Implement with JWT
type JWTSessionManager struct { ... }
```

---

## Architectural Decisions Summary

| Decision | Rationale | Trade-off |
|----------|-----------|-----------|
| In-memory state cache | Simple for MVP | Doesn't scale multi-instance |
| ID Token for user info | Fast, no network call | Requires JWT validation |
| No database in M1 | Scope reduction | Must add later |
| OAuthProvider interface | Extensible to Dropbox, OneDrive | Slightly more indirection |
| Viper for config | Supports env vars + files | One more dependency |
| Crypto/rand for state | Secure | Slightly slower than math/rand |
| HTTPS required | Security best practice | Complicates local dev |

---

## Deliverables Summary

✅ **OAuth Sequence Diagram** - Step-by-step HTTP requests (Section 1)

✅ **Folder Responsibilities** - Every file's purpose (Section 3)

✅ **Service Responsibilities** - Layer breakdown (Section 2)

✅ **Request Lifecycle** - Full callback flow (Section 7.2)

✅ **Error Handling Plan** - Every failure scenario (Section 7.3)

✅ **Security Considerations** - HTTPS, token handling, CSRF (Section 12)

✅ **Configuration Strategy** - Env vars, Viper, secrets (Section 13)

✅ **Future Extensibility Notes** - Multi-provider abstraction (Section 10)

---

## Next Steps: Implementation

Once this design is approved, implementation will proceed in this order:

1. **Configuration Layer** - Load and validate OAuth secrets
2. **Domain Interfaces** - Define OAuthProvider, StateManager contracts
3. **Infrastructure** - Implement GoogleOAuthProvider, state management
4. **Service Layer** - Build AuthService orchestration
5. **Transport Layer** - HTTP handlers and routes
6. **Manual Testing** - Test full OAuth flow with Google
7. **Error Handling** - Implement all error scenarios
8. **Logging** - Add structured logging for observability

Each component will be written to the interface, not the implementation, ensuring testability and future provider support.

