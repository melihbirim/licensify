# Licensify

**The only licensing server that protects your AI API keys.**

_Self-hosted licensing + API key protection for AI-powered applications._

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

## The Problem

Building AI-powered CLI tools or desktop apps? You face two problems:

**1. API Key Problem:**

- 🔑 **OpenAI/Anthropic keys cost $100s/month** - Can't embed them in client apps
- 🚨 **Desktop apps are easily reverse-engineered** - Keys get extracted and abused
- 💸 **One leaked key = unlimited liability** - Your entire API budget stolen

**2. Licensing Problem:**

- 🔐 Need secure license verification without exposing secrets
- 💳 Want free/pro/enterprise tiers with different limits
- 📊 Must track and limit API usage per license

## The Solution

Licensify solves both in one self-hosted deployment:

**🔐 Two Modes of Operation:**

**Mode 1: Direct (Encrypted Key Delivery)**

- Server encrypts and delivers API keys to activated clients
- Client decrypts and uses key locally
- AES-256-GCM encryption
- Best for: Simple CLI tools, desktop apps with local execution

**Mode 2: Proxy (Zero-Trust)**

- ✅ **Keys NEVER leave your server** - True zero-trust architecture
- ✅ **Server-side rate limiting** - Enforce quotas, impossible to bypass
- ✅ **OpenAI + Anthropic support** - Proxy all AI API calls
- ✅ **Per-IP rate limiting** - DDoS protection built-in
- Best for: Production apps, maximum security

**Licensing Features:**
✅ **Ed25519 signatures** - Fast cryptographic verification  
✅ **Multi-tier support** - Free (10 calls/day) → Pro (1000/day) → Enterprise (unlimited)  
✅ **Hardware binding** - Prevent license sharing  
✅ **Usage tracking** - Real-time monitoring per license  
✅ **Email verification** - Free tier with verification flow
✅ **Self-hosted** - Deploy anywhere in 5 minutes  
✅ **Dual database support** - SQLite (dev) or PostgreSQL (production)

## Features

- 🔐 **Two Operation Modes**: Direct (encrypted key delivery) or Proxy (keys never leave server)
- 🔑 **AI API Protection**: OpenAI and Anthropic support with server-side proxying
- 🎚️ **Multi-tier Licensing**: Flexible tier system (tier-1, tier-2, tier-N) with TOML configuration
- 🔄 **Tier Deprecation & Migration**: Seamlessly migrate users from old to new tiers without code changes
- 📊 **Rate Limiting**: Per-license quotas + per-IP DDoS protection
- 🖥️ **Hardware Binding**: Prevent license sharing across devices
- ✉️ **Email Verification**: Free tier with verification flow via Resend
- 💾 **Dual Database Support**: SQLite (dev/small scale) or PostgreSQL (production)
- 🐳 **Docker Ready**: Multi-arch builds (amd64/arm64)
- 📡 **RESTful API**: Simple HTTP endpoints

## 🚀 5-Minute Quickstart

**Get Licensify running in 5 minutes:**

```bash
# 1. Clone and build
git clone https://github.com/yourusername/licensify
cd licensify
make build

# 2. Generate keys
make keygen

# 3. Set up environment
cp .env.example .env
# Edit .env with your keys

# 4. Run with Docker
docker-compose up -d

# 5. Test it works
curl http://localhost:8080/health
```

**Done!** Your licensing server is live at `http://localhost:8080`

---

## Architecture Overview

```mermaid
graph TB
    Client[Client Application]
    CLI[licensify-cli]
    Server[Licensify Server]
    DB[(Database)]
    Email[Email Service]
    AI[AI APIs<br/>OpenAI/Anthropic]
    
    Client -->|License Flow| Server
    CLI -->|License Management| Server
    Server -->|Store| DB
    Server -->|Verify Email| Email
    
    subgraph "Proxy Mode"
        Client -->|API Calls| Server
        Server -->|Proxied| AI
    end
    
    subgraph "Direct Mode"
        Client -.->|Encrypted Key| AI
    end
    
    style Server fill:#4a90e2
    style Client fill:#50c878
    style AI fill:#ff6b6b
```

### Database Schema

```mermaid
erDiagram
    LICENSES ||--o{ ACTIVATIONS : has
    LICENSES ||--|| USAGE : tracks
    
    LICENSES {
        string license_key PK
        string customer_email
        string tier
        int daily_limit
        int monthly_limit
        timestamp expires_at
        timestamp created_at
    }
    
    ACTIVATIONS {
        int id PK
        string license_key FK
        string hardware_id
        timestamp activated_at
        blob encrypted_key
    }
    
    USAGE {
        string license_key PK
        int daily_count
        int monthly_count
        date last_reset_daily
        date last_reset_monthly
    }
    
    VERIFICATION_CODES {
        string email PK
        string code
        string tier
        timestamp expires_at
        timestamp created_at
    }
```

### License State Machine

```mermaid
stateDiagram-v2
    [*] --> Pending: POST /init
    Pending --> Active: POST /verify (with code)
    Active --> Activated: POST /activate (with hardware_id)
    
    Activated --> RateLimited: Daily/Monthly quota exceeded
    RateLimited --> Activated: Quota reset (daily/monthly)
    
    Activated --> Expired: expires_at reached
    Active --> Expired: expires_at reached
    
    Active --> Revoked: Admin revokes license
    Activated --> Revoked: Admin revokes license
    
    Expired --> [*]
    Revoked --> [*]
    
    note right of RateLimited
        Returns 429
        Usage tracked per license
    end note
    
    note right of Activated
        Can make API calls
        Within rate limits
    end note
```

📊 **More Diagrams:** See [docs/diagrams/](docs/diagrams/) for deployment architecture, rate limiting algorithm, and auto-generated code flows.

### License Flow Diagrams

#### 1. Complete License Request Flow

```mermaid
sequenceDiagram
    participant User
    participant CLI as licensify-cli
    participant API as Licensify Server
    participant DB as Database
    participant Email as Email Service
    
    Note over User,Email: Step 1: Request License
    User->>CLI: licensify init
    CLI->>CLI: Prompt for email & tier
    CLI->>API: POST /init
    API->>DB: Create verification code
    API->>Email: Send verification email
    Email-->>User: 📧 6-digit code
    API-->>CLI: Success: code sent
    
    Note over User,Email: Step 2: Verify Email
    User->>CLI: licensify verify --code 123456
    CLI->>API: POST /verify (email, code, tier)
    API->>DB: Validate code & create license
    API->>DB: Generate license key
    API-->>CLI: Return license_key, tier, limits
    CLI->>CLI: Save to ~/.licensify/config.json
    
    Note over User,Email: Step 3: Activate License
    User->>CLI: licensify activate
    CLI->>CLI: Detect hardware ID
    CLI->>API: POST /activate (license_key, hardware_id)
    API->>DB: Validate & store activation
    API-->>CLI: Success + encrypted API bundle
    CLI->>CLI: Store activation locally
```

#### 2. License Check Flow

```mermaid
sequenceDiagram
    participant App as Client App
    participant API as Licensify Server
    participant DB as Database
    participant Cache as Rate Limit Cache
    
    App->>API: POST /check (license_key, hardware_id)
    API->>DB: Validate license
    API->>DB: Check hardware_id match
    API->>Cache: Check rate limits
    
    alt License Valid & Within Limits
        API->>Cache: Increment usage counters
        API-->>App: ✅ Valid (usage, limits, tier)
    else License Invalid
        API-->>App: ❌ Invalid (reason)
    else Rate Limit Exceeded
        API-->>App: ⚠️ Limit exceeded (daily/monthly)
    end
```

#### 3. Proxy Mode Flow (Secure)

```mermaid
sequenceDiagram
    participant App as Client App
    participant Proxy as Licensify Proxy
    participant DB as Database
    participant AI as OpenAI/Anthropic
    
    App->>Proxy: POST /proxy/openai/chat/completions
    Note over App,Proxy: Headers: X-License-Key, X-Hardware-ID
    
    Proxy->>DB: Validate license
    Proxy->>DB: Check rate limits
    
    alt Valid & Within Limits
        Proxy->>AI: Forward request with real API key
        AI-->>Proxy: AI response
        Proxy->>DB: Increment usage counter
        Proxy-->>App: ✅ AI response
    else Invalid License
        Proxy-->>App: ❌ 401 Unauthorized
    else Rate Limit Exceeded
        Proxy-->>App: ⚠️ 429 Too Many Requests
    end
```

#### 4. Direct Mode Flow (Encrypted Key Delivery)

```mermaid
sequenceDiagram
    participant App as Client App
    participant Server as Licensify Server
    participant DB as Database
    participant AI as OpenAI/Anthropic
    
    Note over App,AI: One-time Activation
    App->>Server: POST /activate
    Server->>DB: Validate & store
    Server->>Server: Encrypt API key with AES-256-GCM
    Server-->>App: Encrypted API bundle
    App->>App: Decrypt & store key locally
    
    Note over App,AI: Subsequent API Calls
    App->>AI: Direct API calls with decrypted key
    AI-->>App: Response
    
    Note over App,AI: Periodic License Check
    App->>Server: POST /check (validate license)
    Server->>DB: Check license validity
    Server-->>App: Valid/Invalid status
```

### Getting Your First License

**Complete Flow (3 Steps):**

```bash
# Step 1: Request a license
curl -X POST http://localhost:8080/init \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "tier": "free"}'

# → Check your email for 6-digit verification code

# Step 2: Verify email and create license
curl -X POST http://localhost:8080/verify \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "code": "123456",
    "tier": "free"
  }'

# → Response includes your license key: LIC-XXXX-XXXX-XXXX-XXXX

# Step 3: Activate on your device
curl -X POST http://localhost:8080/activate \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "LIC-XXXX-XXXX-XXXX-XXXX",
    "hardware_id": "your-machine-id"
  }'

# → Returns encrypted API key bundle for your use
```

**Development Mode (Skip Email Verification):**
```bash
# Set in .env
REQUIRE_EMAIL_VERIFICATION=false

# Then skip to verify with any code:
curl -X POST http://localhost:8080/verify \
  -d '{"email": "test@example.com", "code": "any", "tier": "free"}'
```

### 1. Build & Run

```bash
# Build
make build

# Run (Direct Mode - encrypted key delivery)
./licensify

# Run (Proxy Mode - keys never leave server)
PROXY_MODE=true OPENAI_API_KEY=sk-xxx ./licensify
```

### 2. Configuration

Create `.env` file:

```bash
# Required
PRIVATE_KEY=base64_ed25519_private_key  # Generate with tools/keygen.go
PORT=8080

# For Direct Mode (encrypted key delivery)
PROTECTED_API_KEY=your-openai-key-here  # Encrypted and sent to clients

# For Proxy Mode (server-side API calls)
PROXY_MODE=true
OPENAI_API_KEY=sk-xxx     # OpenAI proxy endpoint
ANTHROPIC_API_KEY=sk-xxx  # Anthropic proxy endpoint

# Email verification (free tier)
RESEND_API_KEY=re_xxx
FROM_EMAIL=noreply@yourdomain.com
REQUIRE_EMAIL_VERIFICATION=true  # Set to false for development/self-hosted

# Database
DB_PATH=activations.db         # SQLite (development, small scale)
# DATABASE_URL=postgres://user:pass@host:5432/licensify  # PostgreSQL (production)
```

**Database Options:**

- **SQLite** (default): Perfect for development and small-scale deployments (<1000 licenses)
- **PostgreSQL**: Recommended for production, handles concurrent requests better, required for horizontal scaling

### 3. Choose Your Mode

**Direct Mode (Simple)**

```bash
# Client receives encrypted API key
# Best for: CLI tools, simple desktop apps
./licensify
```

**Proxy Mode (Secure)**

```bash
# Keys stay on server, client uses proxy endpoints
# Best for: Production apps, maximum security
PROXY_MODE=true OPENAI_API_KEY=sk-xxx ./licensify
```

## API Endpoints

### Free Tier Flow (Email Verification)

**1. POST /init** - Request verification code

```json
{ "email": "user@example.com" }
```

**2. POST /verify** - Verify code and get license

```json
{ "email": "user@example.com", "code": "123456" }
```

Returns: `{"success": true, "license_key": "LIC-...", "tier": "free", "daily_limit": 10}`

**3. POST /activate** - Activate license on device

```json
{
  "license_key": "LIC-202512-ABC123-XYZ789",
  "hardware_id": "machine-fingerprint",
  "timestamp": "2025-12-23T10:30:00Z"
}
```

**Direct Mode Response:**

```json
{
  "success": true,
  "encrypted_api_key": "base64_encrypted_data",
  "iv": "base64_iv",
  "limits": { "daily_limit": 10, "monthly_limit": 300 }
}
```

**Proxy Mode Response:**

```json
{
  "success": true,
  "encrypted_api_key": "encrypted_proxy_key_px_xxx", // Use this for proxy calls
  "limits": { "daily_limit": 10, "monthly_limit": 300 }
}
```

### Proxy Mode Endpoints

**Important**: Proxy requests require HMAC-SHA256 signatures for security. See [docs/SECURITY.md](docs/SECURITY.md) for client integration examples.

**POST /proxy/openai/\*** - Proxy OpenAI requests

```json
{
  "proxy_key": "px_generated_from_activation",
  "provider": "openai",
  "body": {
    "model": "gpt-3.5-turbo",
    "messages": [{ "role": "user", "content": "Hello" }]
  },
  "signature": "hmac_sha256_signature",
  "timestamp": 1735689600
}
```

**POST /proxy/anthropic/\*** - Proxy Anthropic requests

```json
{
  "proxy_key": "px_generated_from_activation",
  "provider": "anthropic",
  "body": {
    "model": "claude-3-sonnet-20240229",
    "messages": [{ "role": "user", "content": "Hello" }]
  },
  "signature": "hmac_sha256_signature",
  "timestamp": 1735689600
}
```

Returns OpenAI/Anthropic response with rate limit headers:

- `X-RateLimit-Limit: 10`
- `X-RateLimit-Remaining: 9`
- `X-RateLimit-Reset: 2025-12-24T00:00:00Z`

### Other Endpoints

**POST /usage** - Report usage (direct mode)
**GET /health** - Health check

## Security Features

**🔒 Production-Grade Security (2025 Updates):**

- 🔐 **Argon2id Key Derivation**: Memory-hard encryption with per-license salt (replaces weak SHA256)
- 🔏 **HMAC Request Signing**: Proxy endpoints require cryptographic signatures to prevent key theft
- ⏱️ **Replay Attack Protection**: 5-minute timestamp window on all signed requests
- 🛡️ **Constant-Time Comparison**: Prevents timing attacks on signature validation
- ✅ **Startup Validation**: Server fails fast with clear errors if secrets are missing/invalid
- 📝 **PII Redaction**: Email and license key redaction in logs (GDPR/CCPA compliant)
- 🔄 **PostgreSQL Connection Pooling**: Production-tuned for high concurrency (25 max connections)
- 💾 **SQLite WAL Mode**: ~30% performance improvement with Write-Ahead Logging

**Proxy Mode (Maximum Security):**

- 🔒 API keys NEVER leave server
- 🚦 Server-side rate limiting (impossible to bypass)
- 🛡️ Per-IP rate limiting (10 req/sec, DDoS protection)
- 📊 Usage tracking per license
- 🔐 Unique proxy keys per activation with HMAC signatures

**Direct Mode:**

- 🔐 AES-256-GCM encryption for API keys
- 🔑 Ed25519 signature verification
- 🖥️ Hardware binding prevents license sharing
- ✉️ Email verification for free tier (optional bypass for development)
- 📈 Usage tracking and limits

**General:**

- ⚠️ HTTPS required in production
- 🔒 One free license per hardware device
- 🚫 Configurable activation limits per license
- ✅ Graceful shutdown for zero-downtime deployments

**📚 Security Documentation:**

- See [docs/SECURITY.md](docs/SECURITY.md) for complete security audit and implementation details
- Includes client integration examples for HMAC signing (JavaScript, Python, Go)

## Deployment

### Docker (Recommended)

```bash
# Build
make docker-build

# Run with Direct Mode
docker run -p 8080:8080 --env-file .env licensify:latest

# Run with Proxy Mode
docker run -p 8080:8080 \
  -e PROXY_MODE=true \
  -e OPENAI_API_KEY=sk-xxx \
  --env-file .env \
  licensify:latest
```

### Graceful Shutdown

The server supports graceful shutdown for zero-downtime deployments:

```bash
# Send SIGTERM or SIGINT (Ctrl+C)
kill -TERM <pid>

# Server will:
# 1. Stop accepting new connections
# 2. Complete in-flight requests (up to SHUTDOWN_TIMEOUT)
# 3. Close database connections cleanly
# 4. Exit gracefully
```

**Configuration:**

```env
SHUTDOWN_TIMEOUT=30s  # Time to wait for active requests (default: 30s)
```

This works seamlessly with:

- **Docker**: `docker stop` sends SIGTERM
- **Kubernetes**: Respects termination grace period
- **systemd**: `systemctl stop` sends SIGTERM
- **Cloud platforms**: Rolling deployments without dropped requests

### Cloud Platforms

**Fly.io:**

```bash
fly launch
```

**Railway:**
Connect GitHub repo, set environment variables

**DigitalOcean / AWS / GCP:**
Deploy Docker container with environment variables

### Environment Variables

**Required:**

- `PRIVATE_KEY` - Base64 Ed25519 private key (generate with `tools/keygen.go`)
- `PORT` - Server port (default: 8080)

**Optional:**

- `SHUTDOWN_TIMEOUT` - Graceful shutdown timeout (default: 30s)

**For Direct Mode:**

- `PROTECTED_API_KEY` - API key to encrypt and deliver

**For Proxy Mode:**

- `PROXY_MODE=true`
- `OPENAI_API_KEY` - For OpenAI proxy
- `ANTHROPIC_API_KEY` - For Anthropic proxy

**Email Verification (Free Tier):**

- `RESEND_API_KEY` - Resend API key
- `FROM_EMAIL` - Sender email address

**Database:**

- `DB_PATH` - SQLite path (default: activations.db, for dev/testing)
- `DATABASE_URL` - PostgreSQL URL (recommended for production)
  - Example: `postgresql://user:pass@host:5432/licensify?sslmode=require`

### PostgreSQL Setup (Production)

**Option 1: Managed Database**

- **Fly.io Postgres**: `fly postgres create`
- **Railway**: Add PostgreSQL from dashboard
- **Supabase**: Free tier with connection pooling
- **Neon**: Serverless PostgreSQL

**Option 2: Self-hosted**

```bash
docker run -d \
  -e POSTGRES_PASSWORD=yourpass \
  -e POSTGRES_DB=licensify \
  -p 5432:5432 \
  postgres:16-alpine
```

Set `DATABASE_URL` and server will automatically use PostgreSQL:

```bash
DATABASE_URL=postgresql://user:pass@host:5432/licensify ./licensify
```

Tables are created automatically on first run.

## Database Management

**SQLite:**

```bash
# View licenses
sqlite3 activations.db "SELECT license_id, customer_email, tier, expires_at FROM licenses;"

# Check activations
sqlite3 activations.db "SELECT * FROM activations WHERE license_id='LIC-...';"

# Deactivate license
sqlite3 activations.db "UPDATE licenses SET active=0 WHERE license_id='LIC-...';"

# View proxy keys (proxy mode)
sqlite3 activations.db "SELECT proxy_key, license_id FROM proxy_keys;"
```

**PostgreSQL:**

```bash
# Connect to database
psql $DATABASE_URL

# View licenses
SELECT license_id, customer_email, tier, expires_at FROM licenses;

# Check activations
SELECT * FROM activations WHERE license_id='LIC-...';

# Deactivate license
UPDATE licenses SET active=false WHERE license_id='LIC-...';
```

## Tier Management

Licensify supports flexible tier management with TOML configuration. Define tiers with numeric naming (tier-1, tier-2, tier-N) for easy lifecycle management.

### Configuration

Create `tiers.toml` in your deployment directory:

```toml
[tiers.tier-1]
name = "Free"
daily_limit = 10
monthly_limit = 100
max_devices = 1
features = ["basic_api_access"]
email_verification_required = true
description = "Perfect for trying out the service"

[tiers.tier-2]
name = "Professional"
daily_limit = 1000
monthly_limit = 30000
max_devices = 3
price_monthly = 29.99
description = "For individual developers and small teams"

[tiers.tier-3]
name = "Enterprise"
daily_limit = -1  # -1 means unlimited
monthly_limit = -1
max_devices = -1
price_monthly = 299.99
description = "Unlimited access with dedicated support"
```

Set the config path via environment variable or use default:

```bash
TIERS_CONFIG_PATH=tiers.toml ./licensify
```

### Tier Deprecation & Migration

Deprecate old tiers and automatically migrate users to new ones:

```toml
[tiers.tier-1]
name = "Free (Legacy)"
daily_limit = 10
monthly_limit = 100
max_devices = 1
deprecated = true
migrate_to = "tier-11"  # Migration target

[tiers.tier-11]
name = "Free V2"
daily_limit = 20  # Better limits
monthly_limit = 200
max_devices = 2
description = "New improved free tier"
```

**Admin CLI Commands:**

```bash
# List all tiers (shows deprecated markers)
./licensify-admin tiers list

# Validate tier configuration
./licensify-admin tiers validate

# Get specific tier details
./licensify-admin tiers get tier-1

# Dry-run migration preview
./licensify-admin migrate -from tier-1 -dry-run

# Migrate all users from deprecated tier
./licensify-admin migrate -from tier-1

# Migrate to specific tier (override config)
./licensify-admin migrate -from tier-1 -to tier-2

# Disable email notifications
./licensify-admin migrate -from tier-1 -send-email=false
```

**Migration Process:**

- Validates source and target tiers exist
- Shows preview with limit changes
- Requires confirmation before proceeding
- Updates tier and limits in database
- Optionally sends email to each customer
- Provides detailed success/failure report

**Benefits:**

- No code changes required
- Flexible numeric naming (tier-1, tier-11, tier-100)
- Safe batch migrations with dry-run mode
- Automatic email notifications
- Clear audit trail

📚 **Full Documentation:** [docs/tier-migration.md](docs/tier-migration.md)

## Documentation & Diagrams

### Flow Diagrams

This README includes Mermaid sequence diagrams showing:
- Complete license request flow (init → verify → activate)
- License check flow with rate limiting
- Proxy mode API flow
- Direct mode encrypted key delivery

### Auto-Generated Code Diagrams

Generate flow diagrams automatically from the codebase:

```bash
# Generate all diagrams
make diagrams

# Or run the script directly
./tools/generate-diagrams.sh
```

This creates:
- **API handler call graphs** - Shows function call flows
- **CLI command flows** - Visualizes CLI execution
- **Package structure** - PlantUML class diagrams

Diagrams are saved to `docs/diagrams/` and auto-generated on every push to main via GitHub Actions.

**Tools used:**
- `go-callvis` - Call graph visualization
- `goplantuml` - UML diagrams from Go structs
- Mermaid - Sequence diagrams (this README)

See [docs/diagrams/README.md](docs/diagrams/README.md) for details.

## License

AGPL-3.0 - See [LICENSE](LICENSE) for details.
