# Tier Migration Guide

## Overview

Licensify supports automatic tier deprecation and migration, allowing you to phase out old tiers and seamlessly migrate users to new ones without code changes.

## How It Works

### 1. Tier Deprecation

When you want to deprecate a tier:

1. Add `deprecated = true` to the tier in `tiers.toml`
2. Specify `migrate_to = "tier-X"` to indicate the target tier for migration
3. The tier validation will ensure the migration target exists

Example:

```toml
[tiers.tier-1]
name = "Free (Legacy)"
daily_limit = 10
monthly_limit = 100
max_devices = 1
features = ["basic_api_access", "email_verification"]
email_verification_required = true
description = "Legacy free tier - migrated to tier-11"
deprecated = true
migrate_to = "tier-11"  # Users on tier-1 should be migrated to tier-11

[tiers.tier-11]
name = "Free V2"
daily_limit = 20
monthly_limit = 200
max_devices = 2
features = ["basic_api_access", "email_verification"]
email_verification_required = true
description = "New improved free tier with better limits"
```

### 2. Automatic Tier Resolution

When a deprecated tier is requested:

- **API Endpoint `/tiers`**: Only shows non-deprecated tiers to new customers
- **License Creation**: Automatically uses the migration target tier's limits
- **License Verification**: Works seamlessly - existing licenses continue to function

### 3. Migration Command

The admin CLI provides a `migrate` command to batch-migrate licenses:

```bash
# View migration plan (dry-run)
./licensify-admin migrate -from tier-1 -dry-run

# Perform migration with configured target
./licensify-admin migrate -from tier-1

# Migrate to a specific tier (override config)
./licensify-admin migrate -from tier-1 -to tier-2

# Disable email notifications
./licensify-admin migrate -from tier-1 -send-email=false
```

#### Migration Process:

1. **Validation**: Ensures source and target tiers exist
2. **Query**: Finds all active licenses on the source tier
3. **Preview**: Shows migration plan with limit changes
4. **Confirmation**: Requires "yes" to proceed
5. **Update**: Updates tier and limits in database
6. **Notification**: Sends email to each migrated customer (optional)

#### Email Notifications:

If configured, customers receive a professional email explaining:
- Previous tier details
- New tier details
- Updated limits
- Confirmation that their license key remains the same

Requirements:
- `RESEND_API_KEY` environment variable
- `FROM_EMAIL` environment variable

## Benefits

### For Administrators

- **No Code Changes**: Tier lifecycle management through config only
- **Flexible Naming**: Use tier-1, tier-2, tier-11, tier-100 etc.
- **Safe Migration**: Dry-run mode and confirmation step
- **Automatic Validation**: Prevents invalid migration targets
- **Bulk Operations**: Migrate all licenses at once
- **Customer Communication**: Optional automatic email notifications

### For Customers

- **Seamless Transition**: License keys don't change
- **Improved Limits**: Migrations typically offer better limits
- **Clear Communication**: Email explains what changed
- **No Downtime**: Existing activations continue working

## Validation Rules

The tier system enforces these rules:

1. Migration target must exist
2. Cannot migrate to self
3. Deprecated tier must specify migration target
4. Migration target cannot be deprecated
5. Circular migrations are prevented

## Admin CLI Commands

### List Tiers

```bash
./licensify-admin tiers list
```

Shows all tiers with `[DEPRECATED]` markers and migration targets.

### Validate Configuration

```bash
./licensify-admin tiers validate
```

Checks tier configuration for errors and shows warnings for deprecated tiers.

### Get Tier Details

```bash
./licensify-admin tiers get tier-1
```

Shows details for a specific tier, including deprecation status and migration target.

## Example Workflow

### Scenario: Upgrading Free Tier

1. **Create New Tier** (tier-11) with better limits in `tiers.toml`
2. **Deprecate Old Tier** (tier-1) and point to tier-11
3. **Validate Config**: `./licensify-admin tiers validate`
4. **Test Migration**: `./licensify-admin migrate -from tier-1 -dry-run`
5. **Perform Migration**: `./licensify-admin migrate -from tier-1`
6. **Verify**: Check database and customer feedback

### Scenario: Consolidating Tiers

1. **Mark Multiple Tiers as Deprecated** pointing to single target
2. **Migrate Each Tier** sequentially
3. **Remove Deprecated Tiers** from config after migration complete

## API Behavior

### `/tiers` Endpoint

- Returns only **non-deprecated** tiers
- Used by customers to see available options
- Deprecated tiers are hidden from new signups

### License Activation

- Existing licenses on deprecated tiers continue to work
- New license creation automatically uses migration target limits
- No breaking changes for active users

## Best Practices

1. **Test First**: Always use `-dry-run` before migrating
2. **Backup Database**: Create backup before large migrations
3. **Gradual Rollout**: Deprecate tiers one at a time
4. **Clear Communication**: Ensure emails explain the changes
5. **Monitor Impact**: Check customer support tickets after migration
6. **Keep History**: Don't remove deprecated tiers from config immediately

## Troubleshooting

### Migration Failed

```bash
# Check tier exists
./licensify-admin tiers list

# Validate configuration
./licensify-admin tiers validate

# Check database connection
echo $DATABASE_PATH
```

### No Licenses Found

- Verify tier name is exact (case-sensitive)
- Check licenses are marked as `active = true`
- Ensure database path is correct

### Email Not Sending

- Verify `RESEND_API_KEY` is set
- Verify `FROM_EMAIL` is set
- Check Resend API quotas and status
- Use `-send-email=false` to skip email step

## Technical Details

### Database Schema

Migration updates these fields:
- `tier`: Updated to target tier ID
- `daily_limit`: Updated to target tier's daily limit
- `monthly_limit`: Updated to target tier's monthly limit
- `max_activations`: Updated to target tier's device limit

### Tier Resolution

The `tiers.Get()` function automatically resolves deprecated tiers to their migration targets, while `tiers.GetRaw()` returns the actual tier configuration (used by admin commands).

### Migration Safety

- Transaction-based updates (all-or-nothing)
- Detailed logging of successes and failures
- Confirmation step prevents accidental migrations
- Dry-run mode for testing

---

**Note**: Tier migration is a powerful feature. Always test in a staging environment before production use.
