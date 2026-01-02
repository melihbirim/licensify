# Licensify CLI

A command-line tool for managing Licensify licenses. Request, verify, activate, and check your licenses from the terminal.

## Installation

### From Source

```bash
go install github.com/melihbirim/licensify/cmd/licensify-cli@latest
```

### Build Locally

```bash
make build
# or
go build -o licensify-cli ./cmd/licensify-cli
```

## Quick Start

Get your first license in 3 steps:

```bash
# 1. Request a license
licensify init --email your@email.com --tier free

# 2. Check your email for verification code, then verify
licensify verify --email your@email.com --code 123456 --tier free

# 3. Activate on this machine
licensify activate
```

That's it! Your license is now active.

## Configuration

The CLI stores configuration in `~/.licensify/config.json`:

```json
{
  "server": "http://localhost:8080",
  "license_key": "LIC-xxx-yyy-zzz",
  "hardware_id": "abc123...",
  "tier": "free",
  "expires_at": "2025-01-01T00:00:00Z",
  "activated_at": "2024-01-01T12:00:00Z",
  "last_check": "2024-01-15T10:30:00Z"
}
```

### Environment Variables

Override configuration with environment variables:

- `LICENSIFY_SERVER` - Server URL (default: `http://localhost:8080`)
- `LICENSIFY_KEY` - License key

Example:
```bash
export LICENSIFY_SERVER=https://api.example.com
licensify status
```

## Commands

### `init` - Request a New License

Request a new license by providing your email.

```bash
licensify init --email your@email.com --tier free
```

**Options:**
- `-e, --email` (required) - Your email address
- `-t, --tier` (default: free) - License tier (free, pro, enterprise)

**Output:**
```
✅ Verification code sent!

📧 Check your email: your@email.com

Next step:
  licensify verify --email your@email.com --code <code> --tier free
```

### `verify` - Verify Email and Get License Key

Verify your email with the code sent to you and receive your license key.

```bash
licensify verify --email your@email.com --code 123456 --tier free
```

**Options:**
- `-e, --email` (required) - Your email address
- `-c, --code` (required) - Verification code from email
- `-t, --tier` (default: free) - License tier

**Output:**
```
✅ License created successfully!

License Key: LIC-abc-def-ghi
Customer: your@email.com
Tier: free
Expires: 2025-12-31
Daily Limit: 1000
Monthly Limit: 10000

ℹ License key saved to config

Your license key has also been sent to your email.

Next step:
  licensify activate
```

### `activate` - Activate License on This Machine

Activate your license on the current machine. Hardware ID is automatically detected.

```bash
# Use saved license key
licensify activate

# Or provide key explicitly
licensify activate --key LIC-abc-def-ghi

# Or provide both key and hardware ID
licensify activate --key LIC-abc-def-ghi --hardware-id hw-123
```

**Options:**
- `-k, --key` - License key (uses saved key if omitted)
- `--hardware-id` - Hardware ID (auto-detected if omitted)

**Output:**
```
ℹ Detecting hardware ID...
ℹ Hardware ID: abc123...def (redacted)
ℹ Activating license...
✅ License activated successfully!

License Key: LIC-...ghi (redacted)
Hardware ID: abc123...def (redacted)

Your license is now active!
```

### `status` - Show Local License Status

Display the current license configuration stored locally.

```bash
licensify status
```

**Output:**
```
📋 License Status
─────────────────
License Key:  LIC-...ghi (redacted)
Tier:         free
Server:       http://localhost:8080
Hardware ID:  abc123...def (redacted)
Expires:      2025-12-31
Activated:    2024-01-01 12:00:00
Last Check:   2024-01-15 10:30:00

To check license validity with server:
  licensify check
```

### `check` - Check License Validity with Server

Verify your license status with the server and see usage information.

```bash
# Use saved license key
licensify check

# Or provide key explicitly
licensify check --key LIC-abc-def-ghi
```

**Options:**
- `-k, --key` - License key (uses saved key if omitted)

**Output:**
```
ℹ Checking license with server...
✅ License is valid!

📊 License Details
───────────────────
Status:        ✅ Active
Tier:          free
Customer:      your@email.com
Expires:       2025-12-31 (345 days left)

📈 Usage
────────
Daily:         45 / 1000 (5%)
Monthly:       1234 / 10000 (12%)
```

### `config` - Manage Configuration

View and manage licensify configuration.

#### Show Configuration

```bash
licensify config show
```

**Output:**
```
🔧 Configuration
─────────────────
Server:       http://localhost:8080
License Key:  LIC-...ghi (redacted)
Hardware ID:  abc123...def (redacted)
Tier:         free

Config file:  ~/.licensify/config.json
```

#### Set Configuration Values

```bash
# Set server URL
licensify config set server https://api.example.com

# Set license key
licensify config set key LIC-abc-def-ghi

# Set hardware ID
licensify config set hardware-id hw-123

# Set tier
licensify config set tier pro
```

#### Show Config File Path

```bash
licensify config path
```

**Output:**
```
/Users/username/.licensify/config.json
ℹ (file exists)
Size: 256 bytes
```

#### Export Configuration as JSON

```bash
# Print to console
licensify config export

# Save to file
licensify config export > backup.json
```

#### Reset Configuration

```bash
licensify config reset
```

**Output:**
```
✅ Configuration reset successfully
Deleted: /Users/username/.licensify/config.json
```

## Complete User Journey

Here's a complete example from start to finish:

```bash
# Step 1: Request a license
$ licensify init --email dev@example.com --tier free
✅ Verification code sent!
📧 Check your email: dev@example.com

# Step 2: Check email, then verify with code
$ licensify verify --email dev@example.com --code 123456 --tier free
✅ License created successfully!
License Key: LIC-abc-def-ghi
Customer: dev@example.com
Tier: free
Expires: 2025-12-31
ℹ License key saved to config

# Step 3: Activate on this machine
$ licensify activate
ℹ Detecting hardware ID...
✅ License activated successfully!
Your license is now active!

# Step 4: Check status
$ licensify status
📋 License Status
─────────────────
License Key:  LIC-...ghi (redacted)
Tier:         free
Hardware ID:  abc123...def (redacted)

# Step 5: Verify with server
$ licensify check
✅ License is valid!
📊 License Details
───────────────────
Status:        ✅ Active
Tier:          free
Daily:         0 / 1000 (0%)
Monthly:       0 / 10000 (0%)
```

## Development Mode

If the Licensify server is running in development mode (REQUIRE_EMAIL_VERIFICATION=false), you can skip the init/verify steps:

```bash
# Just activate with any email
licensify activate --key dev@example.com
```

## Hardware ID Detection

The CLI automatically detects your machine's hardware ID:

- **macOS**: Uses IOPlatformSerialNumber or Hardware UUID
- **Linux**: Uses `/etc/machine-id`, `/var/lib/dbus/machine-id`, or DMI UUID
- **Windows**: Uses WMIC csproduct UUID

All hardware IDs are hashed with SHA256 for consistent 64-character format.

## Troubleshooting

### "No license key found"

Run `licensify verify` first to get your license key, or provide it explicitly:

```bash
licensify activate --key LIC-abc-def-ghi
```

### "Failed to detect hardware ID"

Provide the hardware ID manually:

```bash
licensify activate --hardware-id your-hw-id
```

### "License validation failed"

Your license may be expired or revoked. Check status:

```bash
licensify status
licensify check
```

### Change server URL

```bash
licensify config set server https://new-server.com
```

Or use environment variable:

```bash
export LICENSIFY_SERVER=https://new-server.com
```

## Examples

### Using with CI/CD

```bash
# Set license key from secrets
export LICENSIFY_KEY=${{ secrets.LICENSE_KEY }}

# Activate on CI runner
licensify activate

# Check if valid
if licensify check; then
  echo "License valid, proceeding with build"
else
  echo "License invalid, failing build"
  exit 1
fi
```

### Multiple Environments

```bash
# Development
export LICENSIFY_SERVER=http://localhost:8080
licensify activate

# Production
export LICENSIFY_SERVER=https://api.production.com
licensify activate
```

### Backup Configuration

```bash
# Export current config
licensify config export > licensify-backup.json

# Later, restore by setting values
licensify config set key $(jq -r .license_key licensify-backup.json)
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/melihbirim/licensify/issues
- Documentation: https://github.com/melihbirim/licensify/blob/main/README.md
