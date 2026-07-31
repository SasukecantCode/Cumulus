# Milestone 1: Architecture Diagrams & Quick Reference

## OAuth Flow Sequence Diagram

```
User Browser              Cumulus Backend              Google OAuth Server
    |                           |                             |
    | (1) GET /auth/google      |                             |
    |-------------------------->|                             |
    |                           |                             |
    |                    (2) Generate state                   |
    |                    Store in cache                       |
    |                           |                             |
    |<---------- 302 Redirect --|                             |
    | to Google OAuth URL       |                             |
    |                           |                             |
    | (3) Browser redirects to Google, user signs in & grants permission
    |                                                          |
    |<---------- 302 Redirect with code & state -------------|
    |                                                          |
    | (4) GET /auth/google/callback?code=...&state=...       |
    |-------------------------->|                             |
    |                           |                             |
    |                    (5) Validate state                   |
    |                    (accept only valid states)           |
    |                           |                             |
    |                    (6) POST /token                      |
    |                    code + client_secret                 |
    |                           |------------- HTTPS -------->|
    |                           |                             |
    |                           |<--- access/refresh/id token |
    |                           |                             |
    |                    (7) Decode ID Token                  |
    |                    Extract email + Google ID            |
    |                           |                             |
    | (8) Redirect (or JSON response) with user info          |
    |<---------- Success + User Details ---|                 |
    |                           |                             |
```

## Architecture Component Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP Layer (Gin)                         │
│                                                               │
│  ┌───────────────────────────┐      ┌────────────────────┐  │
│  │  /auth/google Handler     │      │  /auth/google/     │  │
│  │                           │      │  callback Handler  │  │
│  │  - Parse requests         │      │                    │  │
│  │  - Call AuthService       │      │  - Extract params  │  │
│  │  - Redirect to OAuth URL  │      │  - Call AuthService│  │
│  │                           │      │  - Return response │  │
│  └───────────┬───────────────┘      └────────┬───────────┘  │
│              │                               │               │
└──────────────┼───────────────────────────────┼───────────────┘
               │                               │
               ▼                               ▼
┌──────────────────────────────────────────────────────────────┐
│              Application Service Layer                        │
│                                                               │
│       ┌────────────────────────────────────┐                │
│       │      AuthService                   │                │
│       │                                    │                │
│       │  - StartGoogleLogin()             │                │
│       │    → Generate state                │                │
│       │    → Call OAuthProvider.GetAuthURL │                │
│       │    → Return OAuth URL              │                │
│       │                                    │                │
│       │  - HandleCallback()                │                │
│       │    → Validate state                │                │
│       │    → Exchange code                 │                │
│       │    → Get user info                 │                │
│       │    → Return UserInfo               │                │
│       └────────────────────────────────────┘                │
│                   │                                          │
│         ┌─────────┴─────────┬──────────────┐                │
│         │                   │              │                │
│         ▼                   ▼              ▼                │
│  ┌─────────────┐   ┌──────────────┐  ┌──────────────┐      │
│  │ StateManager│   │ OAuthProvider│  │   Logger     │      │
│  │(interface)  │   │  (interface) │  │              │      │
│  │             │   │              │  │              │      │
│  │  Methods:   │   │  Methods:    │  │              │      │
│  │ - Generate()│   │ - GetAuthURL()   │              │      │
│  │ - Validate()│   │ - ExchangeCode() │              │      │
│  │             │   │ - GetUserInfo()  │              │      │
│  └─────────────┘   └──────────────┘  └──────────────┘      │
│         │                   │                               │
└─────────┼───────────────────┼───────────────────────────────┘
          │                   │
          ▼                   ▼
┌──────────────────────────────────────────────────────────────┐
│           Infrastructure Adapters                             │
│                                                               │
│  ┌─────────────────────────┐    ┌──────────────────────┐    │
│  │ InMemoryStateManager    │    │ GoogleOAuthProvider  │    │
│  │                         │    │                      │    │
│  │ - Generates secure      │    │ - HTTP calls to      │    │
│  │   random state          │    │   Google APIs        │    │
│  │ - Stores in map with    │    │ - Exchanges code     │    │
│  │   expiration TTL        │    │ - Decodes JWT        │    │
│  │ - Validates on demand   │    │ - Gets user info     │    │
│  │ - One-time use          │    │                      │    │
│  └─────────────────────────┘    └──────────────────────┘    │
│          │                               │                   │
│          ▼                               ▼                   │
│  ┌──────────────┐           ┌──────────────────────┐         │
│  │ In-Memory    │           │ Google Endpoints:    │         │
│  │ Map[string]  │           │                      │         │
│  │ time.Time    │           │ POST /token          │         │
│  │              │           │ GET /userinfo        │         │
│  └──────────────┘           └──────────────────────┘         │
│                                                               │
└──────────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌──────────────────────┐
              │   Google OAuth API   │
              │  (External Service)  │
              │                      │
              │ - /o/oauth2/v2/auth  │
              │ - /token             │
              │ - /userinfo          │
              └──────────────────────┘
```

## Data Flow Diagram: Successful Authentication

```
START: User clicks "Connect Google"
  │
  ├─> Handler: GET /auth/google
  │   ├─> AuthService.StartGoogleLogin()
  │   │   ├─> StateManager.GenerateState() → "abc123xyz..."
  │   │   └─> OAuthProvider.GetAuthURL(state) → "https://accounts.google.com/o/oauth2/v2/auth?..."
  │   └─> Return 302 redirect to Google
  │
  ├─> Browser: Navigates to Google OAuth consent screen
  │   └─> User signs in and clicks "Allow"
  │
  ├─> Google redirects browser to callback
  │   └─> GET /auth/google/callback?code=4/0AY0e-g...&state=abc123xyz...
  │
  ├─> Handler: GET /auth/google/callback
  │   ├─> Extract code and state from query params
  │   ├─> AuthService.HandleCallback(code, state)
  │   │   ├─> StateManager.ValidateState(state)
  │   │   │   ├─> Check: state exists in cache? ✓
  │   │   │   ├─> Check: not expired? ✓
  │   │   │   └─> Delete state (one-time use)
  │   │   │
  │   │   ├─> OAuthProvider.ExchangeCode(code)
  │   │   │   ├─> POST to Google /token endpoint
  │   │   │   ├─> Google validates code, returns tokens
  │   │   │   └─> Return: {access_token, refresh_token, id_token}
  │   │   │
  │   │   ├─> Decode ID Token (JWT)
  │   │   │   ├─> Validate signature using Google's public keys
  │   │   │   └─> Extract: sub (Google ID), email
  │   │   │
  │   │   ├─> OAuthProvider.GetUserInfo(access_token) [optional]
  │   │   │   └─> Fetch from /oauth2/v2/userinfo
  │   │   │
  │   │   └─> Return: UserInfo{email, google_id, refresh_token}
  │   │
  │   └─> Return success response with user details
  │
  └─> END: Display authenticated user info in console
      "Authenticated successfully
       Email: user@gmail.com
       Google Account ID: 118364973454...
       Refresh Token exists: yes"
```

## Error Handling Decision Tree

```
GET /auth/google/callback?code=...&state=...
  │
  ├─ ERROR: No code in params?
  │   └─> Respond: 400 Bad Request "Missing authorization code"
  │
  ├─ ERROR: No state in params?
  │   └─> Respond: 400 Bad Request "Missing state parameter"
  │
  ├─ ERROR: State validation fails (CSRF check)?
  │   ├─ Reason: State not found in cache
  │   ├─ Reason: State expired
  │   ├─ Reason: State already used
  │   └─> Respond: 400 Bad Request "Invalid or expired state"
  │
  ├─ ERROR: Google code exchange fails?
  │   ├─ Reason: Code expired (>10 minutes old)
  │   │   └─> Respond: 400 Bad Request "Authorization code expired"
  │   ├─ Reason: Invalid code (malformed/tampered)
  │   │   └─> Respond: 400 Bad Request "Invalid authorization code"
  │   ├─ Reason: Network timeout
  │   │   └─> Respond: 503 Service Unavailable "Connection failed"
  │   └─ Reason: Google server error
  │       └─> Respond: 503 Service Unavailable
  │
  ├─ ERROR: ID Token decode fails?
  │   ├─ Reason: Invalid JWT signature
  │   │   └─> Respond: 401 Unauthorized "Token validation failed"
  │   └─ Reason: Claims invalid
  │       └─> Respond: 401 Unauthorized "User info invalid"
  │
  └─ SUCCESS: All validations pass
      ├─> Extract: email, google_id from claims
      ├─> Prepare response
      └─> Return: {email, google_id, refresh_token_exists}
```

## Configuration & Environment

```
┌─────────────────────────────────────────────────────────┐
│  Configuration Hierarchy (Top = Highest Priority)       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  1. Environment Variables (CLI override everything)    │
│     GOOGLE_CLIENT_ID=xxx                              │
│     GOOGLE_CLIENT_SECRET=xxx                          │
│     GOOGLE_REDIRECT_URL=http://localhost:8080/...     │
│                                                          │
│  2. .env File (for local development)                  │
│     .env (loaded by viper/godotenv)                    │
│     .gitignore: .env (never commit secrets)            │
│                                                          │
│  3. config.yaml (default fallback)                     │
│     Checked if config file provided at startup         │
│                                                          │
│  4. Hardcoded Defaults (sensible values only)          │
│     HTTP_PORT=8080                                      │
│     LOG_LEVEL=info                                      │
│                                                          │
└─────────────────────────────────────────────────────────┘

Startup Flow:

config.Load()
  ├─> Check for config.yaml
  ├─> Parse and merge with env vars
  ├─> Bind env vars (override file)
  │   GOOGLE_CLIENT_ID
  │   GOOGLE_CLIENT_SECRET
  │   GOOGLE_REDIRECT_URL
  ├─> Validate required fields present
  └─> Return Config struct to main()
```

## Layer Responsibilities at a Glance

```
┌──────────────────────────────────────────────────────────────┐
│ TRANSPORT LAYER (HTTP)                                       │
│ Responsibility: Parse HTTP, call service, format responses   │
│ Files: handlers/auth_handler.go, routes.go                   │
┌──────────────────────────────────────────────────────────────┤
│ SERVICE LAYER (Business Logic)                               │
│ Responsibility: Orchestrate workflows, validate, apply rules │
│ Files: usecase/auth_service.go                               │
├──────────────────────────────────────────────────────────────┤
│ INFRASTRUCTURE (External Integrations)                       │
│ Responsibility: Implement Google OAuth, call Google APIs     │
│ Files: infrastructure/google/oauth.go                        │
├──────────────────────────────────────────────────────────────┤
│ CONFIGURATION (Settings)                                     │
│ Responsibility: Load and validate OAuth secrets              │
│ Files: config/config.go                                      │
├──────────────────────────────────────────────────────────────┤
│ UTILITIES (Cross-cutting)                                    │
│ Responsibility: State generation/validation                  │
│ Files: utils/state.go                                        │
├──────────────────────────────────────────────────────────────┤
│ DOMAIN (Contracts)                                           │
│ Responsibility: Define interfaces (OAuthProvider, etc.)      │
│ Files: domain/ports.go                                       │
└──────────────────────────────────────────────────────────────┘

Key Principle: Each layer has ONE responsibility.
Changes to Google API don't affect handlers.
Changes to HTTP format don't affect services.
```

## Request Lifecycle: Happy Path

```
User Request: GET /auth/google
  │
  ▼
Handler: auth_handler.go/StartLogin()
  │ ├─> Extract provider from URL param (or default "google")
  │ ├─> Call: authService.StartGoogleLogin(provider)
  │ └─> Return: 302 redirect
  │
  ▼
AuthService: auth_service.go/StartGoogleLogin()
  │ ├─> Call: stateManager.GenerateState() → "xyz123"
  │ ├─> Call: oauthProvider.GetAuthURL(state) → full Google URL
  │ └─> Return: URL string
  │
  ▼
Handler: Return 302 redirect
  │ └─> Location: https://accounts.google.com/o/oauth2/v2/auth?...
  │
User grants permission at Google (off-stage)
  │
  ▼
User Browser: Google redirects to callback
  │ └─> GET /auth/google/callback?code=4/0AY0e-g...&state=xyz123
  │
  ▼
Handler: auth_handler.go/Callback()
  │ ├─> Extract: code="4/0AY0e-g...", state="xyz123"
  │ ├─> Call: authService.HandleCallback(code, state)
  │ └─> Return: JSON response with user info
  │
  ▼
AuthService: auth_service.go/HandleCallback()
  │ ├─> Call: stateManager.ValidateState("xyz123")
  │ │   ├─> Check cache: exists? ✓
  │ │   ├─> Check expiry: not expired? ✓
  │ │   └─> Remove from cache (one-time use)
  │ │
  │ ├─> Call: oauthProvider.ExchangeCode(code)
  │ │   ├─> POST https://oauth2.googleapis.com/token
  │ │   │   (body: grant_type=authorization_code, code, client_id, client_secret)
  │ │   └─> Return: {access_token, refresh_token, id_token, expires_in}
  │ │
  │ ├─> Decode ID Token (JWT)
  │ │   ├─> Get Google's public keys
  │ │   ├─> Validate signature
  │ │   └─> Extract claims: sub, email, email_verified
  │ │
  │ └─> Return: UserInfo{GoogleID, Email, RefreshToken exists}
  │
  ▼
Handler: Format response
  │ ├─> Status: 200 OK
  │ └─> Body: {email, google_id, refresh_token_exists, scopes}
  │
  ▼
User/Console: Display authenticated user info
  └─> "Authenticated successfully
       Email: user@gmail.com
       Google Account ID: 118364973454
       Refresh Token exists: yes"
```

## Testing Checklist

```
MANUAL TESTING:
┌─────────────────────────────────────────────────────────┐
│ [ ] Scenario 1: Successful authentication              │
│     • User logs in, grants permission                  │
│     • Backend displays email + Google ID               │
│                                                         │
│ [ ] Scenario 2: User denies permission                │
│     • User clicks "Cancel" at Google                   │
│     • Backend shows: "User denied access"              │
│                                                         │
│ [ ] Scenario 3: State mismatch (CSRF simulation)      │
│     • Manually tamper state param in callback         │
│     • Backend rejects with state validation error      │
│                                                         │
│ [ ] Scenario 4: State expiration                       │
│     • Wait 11+ minutes before callback                 │
│     • Backend rejects with "state expired"             │
│                                                         │
│ [ ] Scenario 5: Invalid authorization code            │
│     • Manually provide invalid code                    │
│     • Backend shows: "Invalid authorization code"      │
│                                                         │
│ [ ] Scenario 6: Network failure                        │
│     • Simulate Google API timeout                      │
│     • Backend shows: "Connection failed"               │
│                                                         │
│ [ ] Scenario 7: Missing required config                │
│     • Start backend without GOOGLE_CLIENT_ID           │
│     • Backend fails fast with clear error              │
│                                                         │
│ [ ] Scenario 8: HTTPS enforcement (production)        │
│     • In production config, verify HTTPS-only         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Security Checklist

```
┌──────────────────────────────────────────────────────┐
│ [ ] CSRF Protection                                  │
│     • State generated with crypto/rand               │
│     • State validated before code exchange           │
│     • State is one-time use                          │
│                                                       │
│ [ ] Token Handling                                  │
│     • Client secret: never logged, never exposed    │
│     • Access token: short-lived, never stored       │
│     • Refresh token: stored encrypted (future)      │
│     • ID token: validated locally                   │
│                                                       │
│ [ ] HTTPS Enforcement                               │
│     • Redirect URL must be HTTPS                    │
│     • Token exchange over HTTPS only                │
│     • Callback must be HTTPS (production)           │
│                                                       │
│ [ ] Code Exchange                                   │
│     • Performed server-to-server only               │
│     • Client secret never exposed to browser        │
│                                                       │
│ [ ] Scope Validation                                │
│     • Verify Google returned expected scopes        │
│     • Reject if scopes mismatch                     │
│                                                       │
│ [ ] Redirect URI Validation                         │
│     • Exact match check (not substring)             │
│     • Must match Google Console exactly             │
│                                                       │
│ [ ] Rate Limiting (future)                          │
│     • Limit login attempts per IP                   │
│     • Prevent brute force / DoS                     │
│                                                       │
│ [ ] Logging                                         │
│     • Never log client secrets                      │
│     • Never log access tokens                       │
│     • Log authentication attempts (for audit)       │
│                                                       │
└──────────────────────────────────────────────────────┘
```

## Future Extensibility: Multi-Provider Template

```
Current (Milestone 1):
  Provider: Google only
  
When adding Dropbox (Milestone 2):

internal/domain/ports.go
  type OAuthProvider interface {          // UNCHANGED
      GetAuthURL(state string) string
      ExchangeCode(ctx, code) (*OAuthTokens, error)
      GetUserInfo(ctx, token) (*UserInfo, error)
  }

internal/infrastructure/
  └── dropbox/
      ├── oauth.go                        // NEW: Implements OAuthProvider
      │   type DropboxOAuthProvider struct { ... }
      │   func (d *DropboxOAuthProvider) GetAuthURL(...) string { ... }
      │   func (d *DropboxOAuthProvider) ExchangeCode(...) { ... }
      │   func (d *DropboxOAuthProvider) GetUserInfo(...) { ... }
      │
      └── scopes.go                       // NEW: Dropbox scope constants

internal/usecase/auth_service.go          // UNCHANGED
  AuthService.HandleCallback() works with ANY provider
  (No "if provider == google/dropbox" logic)

internal/transport/http/handlers/auth_handler.go  // MINIMAL CHANGE
  Accepts provider name from URL param
  Routes based on provider name already exists

main.go                                   // ONLY CHANGE
  providers := map[string]OAuthProvider{
      "google":  &GoogleOAuthProvider{...},
      "dropbox": &DropboxOAuthProvider{...},  // NEW
  }
  authService := NewAuthService(..., providers)
```

---

## Validation Checklist: Before Implementation

- [ ] OAuth sequence understood? (Section 1)
- [ ] Layer responsibilities clear? (Section 2)
- [ ] Project file structure makes sense? (Section 3)
- [ ] Configuration strategy approved? (Section 4 + Diag)
- [ ] State management approach acceptable? (Section 5)
- [ ] Scopes selection justified? (Section 6)
- [ ] Error handling covers all scenarios? (Section 7.3 + Decision Tree)
- [ ] Token handling strategy sound? (Section 8)
- [ ] User identity retrieval method confirmed? (Section 9)
- [ ] Multi-provider extensibility clear? (Section 10 + Template)
- [ ] Testing approach covers MVP? (Section 11 + Checklist)
- [ ] Security considerations addressed? (Section 12 + Checklist)
- [ ] Configuration precedence understood? (Diagram)
- [ ] Current design challenges acknowledged? (Architecture Challenges)

