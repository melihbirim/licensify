# Database Schema and Migrations

This directory contains SQL schema files and migrations for Licensify.

## Structure

```
sql/
├── sqlite/
│   ├── init.sql          # Initial schema for SQLite
│   └── migrations/       # Future migration files
└── postgres/
    ├── init.sql          # Initial schema for PostgreSQL
    └── migrations/       # Future migration files
```

## Initial Schema

The `init.sql` files in each database directory contain the complete initial schema that creates all necessary tables:

- **licenses** - License records with tier, limits, and expiration
- **activations** - Hardware activations for each license
- **verification_codes** - Email verification codes for free tier
- **daily_usage** - Daily usage tracking per license
- **check_ins** - License check-in timestamps
- **proxy_keys** - Proxy mode API keys (if enabled)

## Migrations

Future database schema changes should be added as migration files in the `migrations/` subdirectories.

### Migration Naming Convention

```
YYYYMMDD_HHMMSS_description.sql
```

Example:
```
20260101_120000_add_user_metadata.sql
```

### Migration Best Practices

1. **Idempotent**: Use `IF NOT EXISTS`, `IF EXISTS` clauses
2. **Backward Compatible**: Don't break existing functionality
3. **Tested**: Test on both SQLite and PostgreSQL
4. **Documented**: Include comments explaining the change
5. **Versioned**: Commit migrations to git before deploying

## Running Migrations

Currently, migrations are not automated. The application will:
1. Load the appropriate `init.sql` on first run
2. Future versions will include a migration runner

## Database Differences

### SQLite
- Uses `TEXT` for timestamps (ISO 8601 format)
- Uses `INTEGER` for booleans (0/1)
- Uses `AUTOINCREMENT` for auto-incrementing IDs
- Uses `datetime()` functions for date math

### PostgreSQL
- Uses `TIMESTAMP` for timestamps
- Uses `BOOLEAN` type
- Uses `SERIAL` for auto-incrementing IDs
- Uses `INTERVAL` for date math

Both init.sql files are kept in sync but with appropriate syntax for each database.
