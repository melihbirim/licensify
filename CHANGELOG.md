# Changelog

All notable changes to Licensify will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-01-01

### Added

#### Tier Management System
- **TOML-based tier configuration** - Define tiers in `tiers.toml` without code changes
- **Flexible tier naming** - Use numeric tiers (tier-1, tier-2, tier-N) for easy lifecycle management
- **Tier deprecation** - Mark tiers as deprecated with migration targets
- **Batch migration command** - Migrate all licenses from deprecated tiers with `licensify-admin migrate`
- **Migration dry-run** - Preview changes before performing migrations
- **Email notifications** - Automatic emails to customers during tier migrations
- **Tier validation** - Validates tier configuration and migration targets
- **Admin CLI tier commands** - `tiers list`, `tiers get`, `tiers validate`
- **Migration helpers** - `IsDeprecated()`, `GetMigrationTarget()`, `ListDeprecated()`, `ListActive()`

#### Admin CLI Tool
- **License creation** - Create licenses with custom tiers and limits
- **License upgrade/downgrade** - Upgrade licenses with automatic email notifications
- **License fixes** - Silent corrections to license details without emails
- **List and view licenses** - View all licenses with filtering options
- **Activate/deactivate** - Manage license status
- **Check endpoint** - `/check` endpoint for license status queries

#### Production Readiness
- **Graceful shutdown** - Zero-downtime deployments with proper signal handling
- **Configurable shutdown timeout** - `SHUTDOWN_TIMEOUT` environment variable (default: 30s)
- **Server timeouts** - Read (15s), Write (15s), Idle (60s) timeouts
- **Clean resource cleanup** - Proper database connection closure on shutdown

### Changed
- **Version bump** - Updated from 1.0.0 to 1.1.0
- **Default tier behavior** - Deprecated tiers automatically resolve to migration targets
- **API `/tiers` endpoint** - Only shows non-deprecated tiers to new customers

### Documentation
- **Tier migration guide** - Comprehensive documentation in `docs/tier-migration.md`
- **Admin CLI documentation** - Full command reference in `cmd/licensify-admin/README.md`
- **Updated README** - Added tier management and graceful shutdown sections
- **Environment variables** - Documented `SHUTDOWN_TIMEOUT` and `TIERS_CONFIG_PATH`

### Fixed
- Improved error handling in tier configuration loading
- Better validation for migration targets

## [1.0.0] - 2025-12-22

### Added
- Initial release with core licensing features
- Ed25519 cryptographic signatures
- Two operation modes: Direct (encrypted key delivery) and Proxy (zero-trust)
- Multi-tier licensing support (Free/Pro/Enterprise)
- Hardware binding for license activation
- Email verification for free tier
- PostgreSQL and SQLite database support
- Rate limiting (per-license and per-IP)
- Docker support with multi-arch builds
- OpenAI and Anthropic API proxy support

[1.1.0]: https://github.com/melihbirim/licensify/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/melihbirim/licensify/releases/tag/v1.0.0
