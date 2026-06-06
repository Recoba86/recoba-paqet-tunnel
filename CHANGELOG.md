# Changelog

All notable changes to Recoba Paqet Tunnel are documented in this file.

## v2.1.5 — 2026-06-06

### Release Metadata

- Enforced clean tagged release builds so published binaries carry the expected tag and commit metadata.

## v2.1.4 — 2026-06-06

### Reliability

- Tuned TCP write retry limits for reliability-first tunnel behavior under backpressure.

## v2.1.3 — 2026-06-06

### Health Checks

- Fixed client forward-list port extraction so health checks report missing bound ports accurately.

## v2.1.2 — 2026-06-06

### Health Checks

- Added time-aware runtime health scoping with boot and log-tail fallbacks for more accurate tunnel status.

## v2.1.1 — 2026-06-06

### Updates

- Added robust version extraction and structured backup names for safe core updates.

## v2.1.0 — 2026-05-29

### Operational Features

- **Internal Health Check** — added built-in tunnel health discovery checking active services, configs, binary paths, listening ports, and log files for severe errors. Supports multi-location tunnel checking.
- **Safe Auto-Update** — enhanced core updates with checksum verification, active binary discovery, backup generation, health check integration, and automatic rollback on failure.

## v2.0.0 — 2026-05-29

### Initial Standalone Release

- **Recoba Enhanced Core** — single-core model with ENOBUFS recovery, split raw_packet/tcp_write metrics, 8-retry TCP write backoff, and configurable TX policy.
- **Iran Optimized Profile** — production-validated default preset: KCP MTU 1300, FEC off (pshard/dshard=0), conn=2, window 1536, mode=fast.
- **Multi-Location Tunnels** — one Server A can connect to multiple exit servers simultaneously (Dubai, Switzerland, Germany, etc.) with independent configs and services.
- **Interface Tuning** — automatic PMTU validation, MTU 1492, txqueuelen 4000, fq flow_limit 500p, TCP MSS clamp. All persisted via /etc/rc.local.
- **Migration Tool** — safe migration from old /opt/paqet Paqet Manager installs with backup, dry-run, and rollback support.
- **Passwall Recommendations** — prints recommended client settings (Mux OFF, TFO ON, TLS OFF, RAW TCP) and VLESS URI after setup.
- **Self-Contained Repository** — all source code, build scripts, and installer in one repository.
- **ShellCheck Clean** — 85 test assertions passing, strict ShellCheck validation.

### What This Replaces

This project replaces the multi-provider Paqet Manager with a clean, standalone single-core approach. All third-party provider references (behzad, Paqet-X, Nulled, official) have been removed.
