# Licensify Admin CLI

Command-line tool for managing Licensify licenses.

## Installation

```bash
# Build the admin tool
go build -o licensify-admin ./cmd/licensify-admin/

# Or install globally
go install ./cmd/licensify-admin/
```

## Configuration

The admin tool uses the same database as the main server. Configure via environment variables:

```bash
# SQLite (default)
export DATABASE_PATH=licensify.db

# PostgreSQL
export DATABASE_URL=postgresql://user:pass@localhost:5432/licensify
```

Or create a `.env` file in the same directory.

## Usage

### Create a License

```bash
# Create a Pro license (12 months)
./licensify-admin create \
  -email user@example.com \
  -name "John Doe" \
  -tier pro \
  -months 12

# Create an Enterprise lifetime license
./licensify-admin create \
  -email bigcorp@example.com \
  -name "Big Corp Inc" \
  -tier enterprise \
  -months 0

# Create with custom limits
./licensify-admin create \
  -email custom@example.com \
  -name "Custom User" \
  -tier pro \
  -daily 500 \
  -monthly 15000 \
  -activations 5
```

**Flags:**
- `-email` (required) - Customer email address
- `-name` (required) - Customer name
- `-tier` - License tier: `free`, `pro`, `enterprise` (default: `pro`)
- `-months` - Duration in months, `0` for lifetime (default: `12`)
- `-daily` - Daily API limit, `-1` for unlimited (default: tier-based)
- `-monthly` - Monthly API limit, `-1` for unlimited (default: tier-based)
- `-activations` - Max device activations, `-1` for unlimited (default: tier-based)

**Default Tier Limits:**
- **Free**: 10/day, 100/month, 1 device
- **Pro**: 1000/day, 30000/month, 3 devices
- **Enterprise**: Unlimited

### List Licenses

```bash
# List all licenses
./licensify-admin list

# List only active licenses
./licensify-admin list -active

# Filter by tier
./licensify-admin list -tier pro
```

**Output:**
```
Licenses:
----------------------------------------------------------------------------------------------------
License Key                    Name                 Email                          Tier         Expires      Active
----------------------------------------------------------------------------------------------------
LIC-202512-PRO-446264          John Doe             john@example.com               pro          2026-12-23   ✓
LIC-202512-ENTE-446284         Big Corp             enterprise@bigcorp.com         enterprise   2099-12-31   ✓
----------------------------------------------------------------------------------------------------
Total: 2 licenses
```

### Get License Details

```bash
./licensify-admin get -license LIC-202512-PRO-446264
```

**Output:**
```
License Details:
============================================================
License Key:       LIC-202512-PRO-446264
Customer Name:     John Doe
Customer Email:    john@example.com
Tier:              PRO
Status:            ✅ Active
------------------------------------------------------------
Daily Limit:       1000
Monthly Limit:     30000
Max Activations:   3
Current Activations: 0
------------------------------------------------------------
Created:           2025-12-22 23:31:04
Expires:           2026-12-23 02:31:04
============================================================
```

### Update a License

```bash
# Upgrade to enterprise tier
./licensify-admin update -license LIC-202512-PRO-446264 -tier enterprise

# Update limits
./licensify-admin update -license LIC-202512-PRO-446264 -daily -1 -monthly -1

# Extend license by 6 months
./licensify-admin update -license LIC-202512-PRO-446264 -months 6

# Make it lifetime
./licensify-admin update -license LIC-202512-PRO-446264 -months 0
```

**Flags:**
- `-license` (required) - License key to update
- `-tier` - New tier: `free`, `pro`, `enterprise`
- `-daily` - New daily limit (-1 for unlimited)
- `-monthly` - New monthly limit (-1 for unlimited)
- `-activations` - New max activations (-1 for unlimited)
- `-months` - Extend by N months (0 for lifetime)

### Deactivate/Activate License

```bash
# Deactivate (suspend) a license
./licensify-admin deactivate -license LIC-202512-PRO-446264

# Reactivate a license
./licensify-admin activate -license LIC-202512-PRO-446264
```

## Common Workflows

### New Customer Onboarding

```bash
# 1. Customer signs up for Pro plan
./licensify-admin create \
  -email customer@example.com \
  -name "Customer Name" \
  -tier pro \
  -months 12

# 2. Send them the license key via email
# License Key: LIC-202512-PRO-XXXXXX
```

### Upgrade Customer

```bash
# Customer upgrades from Pro to Enterprise
./licensify-admin update \
  -license LIC-202512-PRO-XXXXXX \
  -tier enterprise \
  -daily -1 \
  -monthly -1 \
  -activations -1
```

### Renewal

```bash
# Extend license by 12 months
./licensify-admin update -license LIC-202512-PRO-XXXXXX -months 12
```

### Suspension

```bash
# Suspend for non-payment
./licensify-admin deactivate -license LIC-202512-PRO-XXXXXX

# Reactivate after payment
./licensify-admin activate -license LIC-202512-PRO-XXXXXX
```

### Custom Enterprise Deal

```bash
# Create custom enterprise license with specific limits
./licensify-admin create \
  -email bigclient@enterprise.com \
  -name "Big Client Inc" \
  -tier enterprise \
  -months 0 \
  -daily 10000 \
  -monthly 300000 \
  -activations 50
```

## Version

```bash
./licensify-admin version
```

## Tips

- Use `-months 0` for lifetime licenses
- Use `-1` for any limit to make it unlimited
- Deactivate licenses instead of deleting them to preserve records
- Use `-active` flag with list to see only active licenses
- Filter by tier when managing large license databases

## Integration with Main Server

The admin tool shares the same database as the main Licensify server:

1. **Database**: Uses same `DATABASE_PATH` or `DATABASE_URL`
2. **Tables**: Reads/writes to `licenses`, `activations` tables
3. **No Server Required**: Direct database access
4. **Safe**: Uses same SQL placeholder logic for PostgreSQL/SQLite compatibility

You can run the admin tool while the server is running - both can access the database concurrently.
