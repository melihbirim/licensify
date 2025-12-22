# Licensify

**The only licensing server that protects your AI API keys.**

*Self-hosted licensing + API key protection for AI-powered applications.*

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
- 🎚️ **Multi-tier Licensing**: Free/Pro/Enterprise with configurable limits
- 📊 **Rate Limiting**: Per-license quotas + per-IP DDoS protection
- 🖥️ **Hardware Binding**: Prevent license sharing across devices
- ✉️ **Email Verification**: Free tier with verification flow via Resend
- 💾 **Dual Database Support**: SQLite (dev/small scale) or PostgreSQL (production)
- 🐳 **Docker Ready**: Multi-arch builds (amd64/arm64)
- 📡 **RESTful API**: Simple HTTP endpoints

## Quick Start

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
{"email": "user@example.com"}
```

**2. POST /verify** - Verify code and get license
```json
{"email": "user@example.com", "code": "123456"}
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
  "limits": {"daily_limit": 10, "monthly_limit": 300}
}
```

**Proxy Mode Response:**
```json
{
  "success": true,
  "encrypted_api_key": "encrypted_proxy_key_px_xxx",  // Use this for proxy calls
  "limits": {"daily_limit": 10, "monthly_limit": 300}
}
```

### Proxy Mode Endpoints

**POST /proxy/openai/*** - Proxy OpenAI requests
```json
{
  "proxy_key": "px_generated_from_activation",
  "provider": "openai",
  "body": {
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }
}
```

**POST /proxy/anthropic/*** - Proxy Anthropic requests
```json
{
  "proxy_key": "px_generated_from_activation",
  "provider": "anthropic",
  "body": {
    "model": "claude-3-sonnet-20240229",
    "messages": [{"role": "user", "content": "Hello"}]
  }
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

**Proxy Mode (Maximum Security):**
- 🔒 API keys NEVER leave server
- 🚦 Server-side rate limiting (impossible to bypass)
- 🛡️ Per-IP rate limiting (10 req/sec, DDoS protection)
- 📊 Usage tracking per license
- 🔐 Unique proxy keys per activation

**Direct Mode:**
- 🔐 AES-256-GCM encryption for API keys
- 🔑 Ed25519 signature verification
- 🖥️ Hardware binding prevents license sharing
- ✉️ Email verification for free tier
- 📈 Usage tracking and limits

**General:**
- ⚠️ HTTPS required in production
- 🔒 One free license per hardware device
- 🚫 Configurable activation limits per license

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

## License

AGPL-3.0 - See [LICENSE](LICENSE) for details.
