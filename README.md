# Recoba Paqet Tunnel

Raw packet tunnel installer and manager for Server A entry nodes and abroad exit servers, with Recoba-enhanced Paqet core builds focused on production tunnel stability.

This project is based on the open-source [Paqet](https://github.com/hanselime/paqet) core and has been independently modified for ENOBUFS recovery, split metrics, TCP write retry backoff, health checks, and safe core updates.

## Repository and Version

- GitHub: https://github.com/Recoba86/recoba-paqet-tunnel
- Latest local tag: `v2.1.9`
- Default installer release tag: `v2.1.9`

## One-Click Install

Run this on both Server A and each exit server:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Recoba86/recoba-paqet-tunnel/main/install.sh)
```

## Roles

- Server A client: user-facing entry server. It listens on local public ports and forwards traffic through one or more Paqet tunnels.
- Exit server: abroad server that receives Paqet tunnel traffic and forwards it to local backend services.

The installer can run both roles from the same script. Use one Server A with multiple independent exit tunnels when you want multiple locations.

## Install an Exit Server

1. Run the installer.
2. Select `1) Setup Server B (Abroad - VPN server)`.
3. Choose the Paqet listen port for this exit server.
4. Enter the backend service ports that should be reachable through the tunnel.
5. Save the generated secret key and public endpoint details securely.
6. Ensure the server firewall allows the chosen Paqet listen port.

Do not commit generated secrets, private keys, or machine-specific configs. Keep those under `.local/` or directly on the server.

## Legacy Install Detection

The manager detects existing Paqet installs before migration:

- Services: `paqet.service`, `paqet-dubai.service`, `paqet-germany.service`, and other `paqet-*.service` units.
- Configs: `/opt/paqet/config*.yaml` and `/opt/recoba-paqet-tunnel/config*.yaml`.
- Binaries: `/opt/paqet/paqet`, `/opt/recoba-paqet-tunnel/paqet`, and `/opt/recoba-paqet-tunnel/recoba-paqet-tunnel`.

`Check Status` and `Safe Diagnostics` show legacy tunnels even when they remain externally managed. `Update/Reinstall Core` parses each active systemd `ExecStart`, backs up the binary actually used by that service, replaces that binary, and restarts only the affected service.

## Manager Menu

```text
Recoba Paqet Tunnel Manager

1) Status & diagnostics
2) Setup / add tunnel
3) Manage existing tunnels
4) Core update
5) Backup / restore
6) Full uninstall / reset this node
0) Exit
```

## Full Uninstall / Reset Node

The reset flow is explicit and only targets Recoba/Paqet-managed items. It stops/disables and removes service units matching `paqet*.service` and `recoba-paqet-tunnel*.service`, removes `/opt/paqet`, `/opt/recoba-paqet-tunnel`, and known temporary Paqet/Recoba files under `/tmp`.

It does not touch `x-ui`, `xray`, `nginx`, `certbot`, unrelated services, user SSH keys, or firewall rules.

Dry-run reset:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Recoba86/recoba-paqet-tunnel/main/install.sh) --reset-node --dry-run
```

Interactive reset with typed confirmation:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Recoba86/recoba-paqet-tunnel/main/install.sh) --reset-node
```

Forced reset without prompt:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Recoba86/recoba-paqet-tunnel/main/install.sh) --yes --reset-node
```

Without `--yes`, you must type:

```text
RESET THIS NODE
```

## Install Server A

1. Run the installer.
2. Select `2) Setup Server A`.
3. Enter a tunnel name such as `dubai` or `germany`.
4. Enter the exit server public address, Paqet port, and generated secret key.
5. Enter the user-facing listen ports that should forward to the exit server backend ports.
6. Repeat the Server A setup for each additional exit location.

Example concept:

```text
Client traffic
    |
    v
Server A
    |-- port 1090 -> recoba-paqet-tunnel-dubai.service   -> Dubai exit server
    |-- port 1091 -> recoba-paqet-tunnel-germany.service -> Germany exit server
```

Each tunnel gets its own config and systemd service:

```text
/opt/recoba-paqet-tunnel/config-<name>.yaml
recoba-paqet-tunnel-<name>.service
```

## Multi-Location Support

Multi-location support is built around independent named tunnels. A practical setup is Dubai on one user-facing port and Germany on another, both managed from the same Server A. Each location can be restarted, checked, tuned, and edited independently.

## Client Settings

Recommended Passwall or raw TCP client settings:

- Mux: off
- TCP Fast Open: on
- TLS: off
- Transport: raw TCP
- MPTCP: off
- Pre-connections: `0`

## Useful Commands

```bash
# Open the manager menu
recoba-paqet-tunnel

# Check service state
systemctl status 'recoba-paqet-tunnel*' --no-pager

# Inspect tunnel logs
journalctl -u recoba-paqet-tunnel-<name>.service --no-pager -n 100

# Check ENOBUFS and retry metrics
journalctl -u recoba-paqet-tunnel-<name>.service --no-pager -n 300 | grep -E 'raw_packet|tcp_write|ENOBUFS|retry'
```

## Local Deployment Helpers

Use `scripts/check-exit-server.sh` with an untracked environment file:

```bash
mkdir -p .local
cat > .local/exit-server.env <<'ENV'
EXIT_SERVER_HOST=203.0.113.10
EXIT_SERVER_USER=root
EXIT_SERVER_PORT=22
EXIT_SERVER_KEY_FILE=.local/keys/exit-server.key
ENV

bash scripts/check-exit-server.sh .local/exit-server.env
```

The `.local/` directory, private key files, environment files, and benchmark output are intentionally ignored by git.

## Configuration Templates

Generic Paqet examples are kept in:

- `core/example/client.yaml.example`
- `core/example/server.yaml.example`

Use the installer for production configs whenever possible because it validates interfaces, ports, secret keys, and service units.

## Migration from Old Paqet Manager

If you have an existing install at `/opt/paqet/`, run:

```text
recoba-paqet-tunnel -> m) Import existing installation / migrate old /opt/paqet
```

The import screen is read-only by default. Optional migration copies configs, creates Recoba Paqet Tunnel service units, and installs the enhanced core without deleting the old setup.

## Build Release Assets

Release assets are built from clean, tagged commits:

```bash
bash scripts/build_release.sh v2.1.9
```

The script builds canonical `recoba-paqet-tunnel-linux-amd64.tar.gz` and `recoba-paqet-tunnel-linux-arm64.tar.gz` tarballs plus `SHA256SUMS` under `build/`. The installer can still fall back to older `recoba-tunnel-linux-*.tar.gz` release assets when needed.

## License

This project is based on the open-source Paqet core. See [LICENSE](LICENSE) for details.
