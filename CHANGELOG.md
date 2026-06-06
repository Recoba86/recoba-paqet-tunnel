# Changelog

## [v2.1.10] - 2026-06-06

### ⚠️ BREAKING CHANGE ⚠️
**Protocol Change**: The internal TCP stream serialization has been switched from `gob` to length-prefixed `json` to fix handshake deadlocks under heavy packet loss and strict Kernel conditions. 
**This version is NOT backward compatible with older versions.** Both server and client must be upgraded to v2.1.10 simultaneously.

### Added
- **Connection Protection**: Idempotent `iptables` rules (NOTRACK and RST DROP) are now automatically managed by the installer for both client and server nodes to prevent kernel RST leakage and Deep Packet Inspection (DPI) drops.
- **Diagnostics**: Added 'Verify connection protection' and 'Reapply connection protection' options to the installer's diagnostic menu.

### Changed
- **TCP Handshake Evasion**: Default `local_flag` is now `["S", "A", "PA"]` and `remote_flag` is `["SA", "PA"]`. This properly simulates a 3-way TCP handshake to maintain NAT mappings and evade connection timeouts in strict environments.
# Changelog

All notable changes to Recoba Paqet Tunnel are documented in this file.

## v2.1.9 — 2026-06-06

### TCP Payload Return Path

- Fixed Paqet TCP bridge lifecycle handling so one copy direction finishing normally no longer closes both sides before the opposite direction can deliver pending payload.
- Added safe bidirectional TCP copy handling with `CloseWrite` support for TCP endpoints and ordered close signaling for stream endpoints that do not support half-close.
- Added regression coverage for HTTP response delivery through the Paqet TCP bridge, including the case where the client request side finishes before the response is returned.

## v2.1.8 — 2026-06-06

### Manager UX and Reset

- Reworked the top-level manager menu into focused sections for status, setup, tunnel management, core update, backup/restore, and full node reset.
- Added explicit full uninstall/reset flow with `RESET THIS NODE` confirmation, dry-run support, and exact cleanup targets for Paqet/Recoba services, unit files, install paths, and known temp files.
- Added non-interactive flags: `--help`, `--reset-node`, `--uninstall`, `--dry-run`, and `--yes --reset-node`.
- Added backup/restore menu for Paqet/Recoba configs, services, and binaries.
- Added regression coverage for reset targets, safety exclusions, dry-run output, and CLI flags.

## v2.1.7 — 2026-06-06

### Legacy Import and Diagnostics

- Fixed manager version display so the installer banner reports `v2.1.7`.
- Added legacy discovery for `/opt/paqet/config*.yaml`, `/opt/paqet/paqet`, and `paqet*.service` units.
- Updated status, diagnostics, and health checks to show legacy and new tunnels before migration.
- Updated safe core updates to parse each active service `ExecStart`, replace the binary it actually uses, back it up, and restart only the owning service.
- Added safer port-conflict handling that identifies known Paqet tunnel owners before setup proceeds.

## v2.1.6 — 2026-06-06

### Release Assets

- Updated the installer to prefer canonical `recoba-paqet-tunnel-linux-*.tar.gz` release assets and fall back to legacy `recoba-tunnel-linux-*.tar.gz` assets.
- Kept release builds on the canonical Recoba Paqet Tunnel asset names.

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
