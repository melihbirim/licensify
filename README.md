# Licensify

**The only licensing server that protects your AI API keys.**

*Community Edition - Self-hosted licensing + API key protection for AI-powered applications.*

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

## The Problem

You're building an AI-powered CLI tool, desktop app, or browser extension. You have two critical problems:

**1. Licensing Problem:**
- 🔐 Need secure license verification without exposing secrets
- 💳 Want free/pro/enterprise tiers with different limits
- 🖥️ Need to prevent license sharing across devices
- 📊 Must track and limit API usage per license

**2. API Key Problem (The Big One):**
- 🔑 **OpenAI/Anthropic keys cost $100s/month** - Can't embed them in client apps
- 🚨 **Desktop apps are easily reverse-engineered** - Keys get extracted and abused
- ⚠️ **CLI tools expose keys in memory** - No safe way to bundle expensive API keys
- 💸 **One leaked key = unlimited liability** - Your entire API budget stolen

Most developers either:
- ❌ Avoid building AI desktop/CLI tools entirely
- ❌ Use insecure key obfuscation (which fails)
- ❌ Build complex backend proxies from scratch (weeks of work)
- ❌ Skip licensing and hope for the best

## The Solution

Licensify solves both problems in one self-hosted deployment:

**AI API Key Protection (Our Superpower):**
✅ **Server-side key storage** - OpenAI/Anthropic keys never touch client devices  
✅ **Encrypted delivery** - AES-256-GCM encryption for activated clients only  
✅ **Proxy mode (coming)** - Issue #1: Keys never leave server, you proxy all API calls  
✅ **Rate limiting** - Enforce API quotas server-side, impossible to bypass  

**Licensing (Because You Need Both):**
✅ **Ed25519 signatures** - Fast cryptographic verification for CLI tools  
✅ **Multi-tier support** - Free (10 calls/day) → Pro (1000/day) → Enterprise (unlimited)  
✅ **Hardware binding** - Prevent license sharing via device fingerprinting  
✅ **Usage tracking** - Real-time API call monitoring per license  
✅ **Self-hosted** - Deploy on Fly.io, Railway, or your VPS in 5 minutes  
✅ **Zero dependencies** - Single binary with SQLite, no MongoDB/Redis needed  

**Deploy once, protect your AI API keys forever.** Stop worrying about leaked OpenAI keys costing you thousands.

## Why Licensify?

Most open-source licensing solutions focus *only* on license verification. Licensify combines **cryptographic licensing** with **API key protection**, making it unique in the ecosystem. Here's how it compares:

### Comparison Matrix

| Feature | Licensify | f-license | lime | licensecc | Standard.Licensing |
|---------|-----------|-----------|------|-----------|-------------------|
| **Language** | Go | Go | Go | C++ | C# |
| **Cryptography** | Ed25519 | RSA/JWT/HMAC | Ed25519 | Ed25519 | RSA |
| **API Key Protection** | ✅ Built-in vault | ❌ | ❌ | ❌ | ❌ |
| **Multi-tier Support** | ✅ Yes | ❌ | ✅ Yes | ❌ | ❌ |
| **Hardware Binding** | ✅ Yes | ❌ | ❌ | ✅ Yes | ❌ |
| **Usage Tracking** | ✅ Built-in | ❌ | ❌ | ❌ | ❌ |
| **Docker Ready** | ✅ Multi-arch | ❌ | ✅ Yes | ❌ | ❌ |
| **Framework Specific** | ❌ Standalone | ❌ Standalone | ❌ Standalone | ❌ Standalone | ✅ .NET required |
| **Database** | SQLite (built-in) | MongoDB required | SQLite | None | None |
| **Proxy Mode** | 🔜 Planned (issue #1) | ❌ | ❌ | ❌ | ❌ |
| **License Type** | AGPL-3.0 | Apache 2.0 | MIT | BSD | MIT |

### Unique Value Propositions

1. **AI API Key Security (Zero Competition)** - Licensify is the ONLY open-source licensing solution that protects expensive AI API keys (OpenAI: $20-200/month, Anthropic: $15-150/month). Your keys stay server-side, encrypted in transit to activated clients.

2. **Purpose-Built for AI Tools** - Designed specifically for CLI tools, desktop apps, and extensions powered by GPT-4, Claude, Gemini. Not a generic license server retrofitted for AI.

3. **Proxy Mode Roadmap** - Issue #1 proposes true zero-trust: Licensify makes API calls on behalf of clients. Keys NEVER leave your server. Rate limiting built-in.

4. **Ed25519 Performance** - Fast signature verification for CLI tools (critical for good UX). Smaller keys than RSA, modern cryptographic security.

5. **Zero Dependencies** - Single binary with SQLite embedded. Deploy in 5 minutes. No MongoDB, PostgreSQL, or Redis needed.

6. **Multi-tier AI Quotas** - Free: 10 calls/day, Pro: 1000/day, Enterprise: unlimited. Limits enforced server-side, impossible to bypass.

7. **Community Edition + Future SaaS** - Self-host now (free, open source). Managed SaaS version coming if community adoption proves demand.

### When to Choose Licensify

**Perfect for:**
- ✅ **AI-powered CLI tools** - GPT-powered dev tools, code generators, AI assistants
- ✅ **Desktop AI apps** - Electron/Tauri apps with OpenAI/Anthropic integration
- ✅ **Browser extensions** - Chrome/Firefox extensions calling AI APIs
- ✅ **Indie hackers** - Building AI wrappers, need licensing + key protection
- ✅ **AI SaaS** - Want self-hosted licensing before scaling to managed service

**Consider alternatives if:**
- ❌ You don't use AI APIs - Use f-license or lime for basic licensing
- ❌ You need C++ integration - Use licensecc for native apps
- ❌ You're building on .NET only - Use Standard.Licensing
- ❌ You need enterprise features today - Wait for SaaS version or fork community edition

## Features

- 🔐 Ed25519 cryptographic license verification
- 🎚️ Multi-tier licensing (free/pro/enterprise)
- 🔑 Server-side API key protection with encrypted client storage
- 📊 Usage tracking and rate limiting
- 🖥️ Hardware-based device fingerprinting
- ✉️ Email verification via Resend
- 💾 SQLite database for persistence
- 🐳 Docker support with multi-arch builds (amd64/arm64)
- 🛠️ Makefile build system with version injection
- 📡 RESTful API endpoints

## Quick Start

### Using Make (Recommended)

```bash
# Build the server
make build

# Run locally
make run

# Build and run with Docker
make docker-build
make docker-run
```

### Manual Setup

1. **Generate Ed25519 keypair:**

   ```bash
   cd tools
   go run keygen.go
   ```

   This creates `public.key` and `private.key`.

2. **Configure environment:**

   ```bash
   cp .env.example .env
   # Edit .env and add:
   # - PRIVATE_KEY (from keygen.go)
   # - PROTECTED_API_KEY (your API key to protect, e.g., OpenAI key)
   # - RESEND_API_KEY (for email verification)
   # - FROM_EMAIL (sender email address)
   ```

3. **Run the server:**
   ```bash
   make build
   ./licensify
   # Server starts on http://localhost:8080
   ```

### Docker Deployment

```bash
# Build with version info
make docker-build

# Or multi-arch build
make docker-build-multi

# Run container
docker run -p 8080:8080 --env-file .env licensify:latest
```

## Creating Licenses

Use the license generation tool:

```bash
cd tools
go run license-gen.go \
  -name="Acme Corporation" \
  -email="billing@acme.com" \
  -months=12 \
  -scans=10000 \
  -activations=3
```

This will:

- Generate a license key (e.g., `LIC-202512-ABC123-XYZ789`)
- Insert it into the database
- Display the key to send to customer

## API Endpoints

### POST /activate

Activate a license key.

**Request:**

```json
{
  "license_key": "LIC-202512-ABC123-XYZ789",
  "hardware_id": "machine-fingerprint",
  "timestamp": "2025-12-18T10:30:00Z"
}
```

**Response (Success):**

```json
{
  "success": true,
  "customer_name": "Acme Corporation",
  "expires_at": "2026-12-18T23:59:59Z",
  "tier": "free",
  "encrypted_api_key": "base64_encrypted_data",
  "iv": "base64_iv",
  "limits": {
    "daily_limit": 10,
    "monthly_limit": 300,
    "max_activations": 3
  }
}
```

**Response (Error):**

```json
{
  "success": false,
  "error": "License has expired"
}
```

### GET /health

Health check endpoint.

**Response:**

```json
{
  "status": "ok",
  "service": "licensify"
}
```

## Database Schema

```sql
CREATE TABLE licenses (
    license_id TEXT PRIMARY KEY,
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    max_scans_per_month INTEGER NOT NULL,
    max_activations INTEGER NOT NULL,
    active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE activations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    license_id TEXT NOT NULL,
    hardware_id TEXT NOT NULL,
    activated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (license_id) REFERENCES licenses(license_id)
);
```

## Security

- **Generic API Key Protection**: Encrypts and delivers any API key securely to authorized clients
- **Ed25519 Signatures**: License keys signed with Ed25519 for cryptographic verification
- **Hardware Binding**: Prevents license sharing via device fingerprinting
- **AES-256-GCM Encryption**: All sensitive data encrypted
- **Free Tier Protection**: One free license per hardware device
- **Activation Limits**: Configurable per license tier
- **Email Verification**: Free tier requires email ownership proof
- **HTTPS Required**: Production deployments must use TLS

## How It Works

1. **Server Deployment**: You deploy your own instance with your private key and API key
2. **Client Integration**: Your client app embeds the corresponding public key
3. **License Verification**: Clients verify licenses signed by YOUR private key only
4. **Key Protection**: Your protected API key is encrypted and delivered only to activated clients
5. **Isolation**: Each deployment is independent - users can't cross-contaminate

This means anyone can run their own activation server for their own app, but licenses from different deployments are incompatible.

## Deployment

Recommended platforms:

- **Fly.io**: `fly launch`
- **Railway**: Connect GitHub repo
- **DigitalOcean App Platform**: Docker deployment
- **AWS Lambda**: Serverless option

Environment variables required:

- `PORT` (default: 8080)
- `PRIVATE_KEY` (base64 Ed25519 private key)
- `PROTECTED_API_KEY` (API key you want to protect - OpenAI, Stripe, etc.)
- `DB_PATH` (default: activations.db)
- `RESEND_API_KEY` (for email verification in free tier)
- `FROM_EMAIL` (sender address for verification emails)

## Monitoring

Monitor these metrics:

- Activation requests per hour
- Failed activation attempts
- Active licenses count
- Database size

## Support Operations

**View all licenses:**

```bash
sqlite3 activations.db "SELECT * FROM licenses;"
```

**View activations for a license:**

```bash
sqlite3 activations.db "SELECT * FROM activations WHERE license_id='LIC-...';"
```

**Deactivate a license:**

```bash
sqlite3 activations.db "UPDATE licenses SET active=0 WHERE license_id='LIC-...';"
```

**Reset activations (for license transfer):**

```bash
sqlite3 activations.db "DELETE FROM activations WHERE license_id='LIC-...';"
```
