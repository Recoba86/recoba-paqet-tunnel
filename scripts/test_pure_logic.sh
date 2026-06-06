#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export PAQET_TEST_MODE=1
# shellcheck source=../install.sh
source "$REPO_ROOT/install.sh"

pass_count=0

assert_eq() {
    local expected="$1"
    local actual="$2"
    local label="$3"

    if [ "$expected" != "$actual" ]; then
        printf 'FAIL: %s\nExpected: %s\nActual:   %s\n' "$label" "$expected" "$actual" >&2
        exit 1
    fi

    pass_count=$((pass_count + 1))
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local label="$3"

    if [[ "$haystack" != *"$needle"* ]]; then
        printf 'FAIL: %s\nMissing: %s\nIn: %s\n' "$label" "$needle" "$haystack" >&2
        exit 1
    fi

    pass_count=$((pass_count + 1))
}

assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local label="$3"

    if [[ "$haystack" == *"$needle"* ]]; then
        printf 'FAIL: %s\nUnexpected: %s\nIn: %s\n' "$label" "$needle" "$haystack" >&2
        exit 1
    fi

    pass_count=$((pass_count + 1))
}

assert_success() {
    local label="$1"
    shift

    if ! "$@"; then
        printf 'FAIL: %s\n' "$label" >&2
        exit 1
    fi

    pass_count=$((pass_count + 1))
}

assert_failure() {
    local label="$1"
    shift

    if "$@" >/dev/null 2>&1; then
        printf 'FAIL: %s\nExpected command to fail.\n' "$label" >&2
        exit 1
    fi

    pass_count=$((pass_count + 1))
}

tmp_dir="$(mktemp -d)"
cleanup() {
    rm -rf "$tmp_dir"
}
trap cleanup EXIT

PAQET_DIR="$tmp_dir/opt-paqet"
PAQET_CONFIG="$PAQET_DIR/config.yaml"
PAQET_BIN="$PAQET_DIR/paqet"
AUTO_RESET_CONF="$PAQET_DIR/auto-reset.conf"
AUTO_RESET_SCRIPT="$PAQET_DIR/auto-reset.sh"
CORE_PROVIDER_META="$PAQET_DIR/core-provider.env"
CORE_PROFILE_META="$PAQET_DIR/core-profile.env"
CORE_INSTALLED_META="$PAQET_DIR/core-installed.env"
PAQET_CORE_CACHE_DIR="$PAQET_DIR/core-cache"
PAQET_CORE_CACHE_ARCHIVE_DIR="$PAQET_CORE_CACHE_DIR/archives"

normalized=""
normalize_forward_mappings_input "443 8443:443, 51820/udp" normalized "tcp"
assert_eq "443,8443:443,51820/udp" "$normalized" "normalize mixed separators and protocols"

normalized=""
normalize_forward_mappings_input "1090:443" normalized "udp"
assert_eq "1090:443/udp" "$normalized" "default UDP protocol is applied"

assert_failure "duplicate listen/protocol pairs are rejected" normalize_forward_mappings_input "443,443/tcp" normalized "tcp"

assert_eq "1090" "$(mapping_listen_port "1090:443/udp")" "mapping_listen_port"
assert_eq "443" "$(mapping_target_port "1090:443/udp")" "mapping_target_port"
assert_eq "udp" "$(mapping_protocol "1090:443/udp")" "mapping_protocol udp"
assert_eq "tcp" "$(mapping_protocol "1090:443")" "mapping_protocol tcp default"

assert_eq "1200" "$(calculate_safe_kcp_mtu "1300" "1300")" "safe MTU leaves 100 bytes on 1300 interface"
assert_eq "1300" "$(calculate_safe_kcp_mtu "1300" "1500")" "safe MTU keeps profile ceiling on 1500 interface"
assert_eq "1300" "$(calculate_safe_kcp_mtu "1300" "")" "safe MTU keeps default when interface MTU missing"
assert_eq "1200" "$(calculate_safe_kcp_mtu "invalid" "1500")" "safe MTU handles invalid profile MTU"
assert_eq "900" "$(calculate_safe_kcp_mtu "1300" "980")" "safe MTU clamps very low interfaces to lower bound"

forward_config=""
build_forward_config_from_mappings_csv "1090:443,51820/udp" forward_config
assert_contains "$forward_config" 'listen: "0.0.0.0:1090"' "forward config TCP listen"
assert_contains "$forward_config" 'target: "127.0.0.1:443"' "forward config TCP target"
assert_contains "$forward_config" 'protocol: "tcp"' "forward config TCP protocol"
assert_contains "$forward_config" 'listen: "0.0.0.0:51820"' "forward config UDP listen"
assert_contains "$forward_config" 'protocol: "udp"' "forward config UDP protocol"

assert_success "valid tunnel name" is_valid_tunnel_name "dubai-1"
assert_failure "uppercase tunnel name is rejected" is_valid_tunnel_name "Dubai"
assert_failure "leading hyphen tunnel name is rejected" is_valid_tunnel_name "-dubai"
assert_failure "long tunnel name is rejected" is_valid_tunnel_name "abcdefghijklmnopqrstuvwxyzabcdefg"

# Single-core model: always returns recoba-enhanced
assert_eq "recoba-enhanced" "$(get_current_core_provider)" "single core provider is recoba-enhanced"

label=$(get_core_provider_label)
assert_contains "$label" "Recoba" "core provider label contains Recoba"

set_current_profile_preset "iran-optimized"
assert_eq "iran-optimized" "$(get_current_profile_preset)" "iran-optimized profile round-trip"

detect_total_mem_mb() { echo "1024"; }
detect_cpu_cores() { echo "1"; }
detect_interface_mtu() { [ "${1:-}" = "eth0" ] && echo "1300"; }

set_current_profile_preset "default"
assert_eq "1200" "$(get_effective_profile_kcp_mtu_for_interface "eth0")" "effective profile MTU adapts to interface MTU"

PROFILE_PRESET_NAME="default"
AUTO_TUNE_CONN="8"
AUTO_TUNE_SNDWND="8192"
AUTO_TUNE_RCVWND="8192"
AUTO_TUNE_SMUXBUF="33554432"
AUTO_TUNE_STREAMBUF="16777216"
apply_low_mtu_upload_stability_profile "900"
assert_eq "4" "$AUTO_TUNE_CONN" "low-MTU profile caps conn"
assert_eq "3072" "$AUTO_TUNE_SNDWND" "low-MTU profile caps send window"
assert_eq "3072" "$AUTO_TUNE_RCVWND" "low-MTU profile caps receive window"
assert_eq "16777216" "$AUTO_TUNE_SMUXBUF" "low-MTU profile caps smux buffer"
assert_eq "8388608" "$AUTO_TUNE_STREAMBUF" "low-MTU profile caps stream buffer"

AUTO_TUNE_CONN="4"
AUTO_TUNE_SNDWND="4096"
apply_low_mtu_upload_stability_profile "1500"
assert_eq "4" "$AUTO_TUNE_CONN" "normal-MTU profile keeps conn"
assert_eq "4096" "$AUTO_TUNE_SNDWND" "normal-MTU profile keeps send window"

# Iran-optimized profile: FEC off, fixed windows, MTU 1200
config_opt="$tmp_dir/config-optimized.yaml"
cat > "$config_opt" <<'YAML'
role: "client"
network:
  interface: "eth0"
transport:
  protocol: "kcp"
  conn: 4
  kcp:
    mode: "fast"
    key: "secret"
    mtu: 1300
    block: "aes"
    sndwnd: 2048
    rcvwnd: 2048
    dshard: 10
    pshard: 3
YAML

apply_profile_preset_to_config_file "$config_opt" "iran-optimized"
assert_contains "$(cat "$config_opt")" '    mode: "fast"' "iran-optimized profile mode"
assert_contains "$(cat "$config_opt")" "    mtu: 1200" "iran-optimized profile MTU (safe: 1300-100 headroom)"
assert_contains "$(cat "$config_opt")" "    sndwnd: 3072" "iran-optimized profile sndwnd 3072"
assert_contains "$(cat "$config_opt")" "    rcvwnd: 3072" "iran-optimized profile rcvwnd 3072"
assert_contains "$(cat "$config_opt")" "    dshard: 0" "iran-optimized profile FEC off (dshard=0)"
assert_contains "$(cat "$config_opt")" "    pshard: 0" "iran-optimized profile FEC off (pshard=0)"
assert_contains "$(cat "$config_opt")" "  conn: 4" "iran-optimized profile conn=4"

# Legacy installation discovery and service-aware updates.
legacy_root="$tmp_dir/legacy-paqet"
new_root="$tmp_dir/recoba-paqet"
SYSTEMD_SYSTEM_DIR="$tmp_dir/systemd"
LEGACY_PAQET_DIR="$legacy_root"
LEGACY_PAQET_BIN="$legacy_root/paqet"
PAQET_DIR="$new_root"
PAQET_BIN="$new_root/recoba-paqet-tunnel"
PAQET_COMPAT_BIN="$new_root/paqet"
mkdir -p "$LEGACY_PAQET_DIR" "$PAQET_DIR" "$SYSTEMD_SYSTEM_DIR"

cat > "$LEGACY_PAQET_DIR/config-dubai.yaml" <<'YAML'
role: "client"
forward:
  - listen: "0.0.0.0:1090"
    target: "127.0.0.1:1090"
    protocol: "tcp"
server:
  addr: "203.0.113.10:9998"
transport:
  protocol: "kcp"
  kcp:
    key: "test-key"
YAML

cat > "$SYSTEMD_SYSTEM_DIR/paqet-dubai.service" <<EOF
[Unit]
Description=legacy paqet dubai

[Service]
ExecStart=$LEGACY_PAQET_BIN run -c $LEGACY_PAQET_DIR/config-dubai.yaml
EOF

cat > "$LEGACY_PAQET_BIN" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "version" ]; then
    echo "Version: v2.1.5"
fi
EOF
chmod +x "$LEGACY_PAQET_BIN"

systemctl() {
    case "${1:-}" in
        cat)
            cat "$SYSTEMD_SYSTEM_DIR/${2%.service}.service"
            ;;
        is-active)
            [ "${2:-}" = "--quiet" ] && [ "${3:-}" = "paqet-dubai" ]
            ;;
        restart)
            echo "$2" >> "$tmp_dir/restarted-services.log"
            ;;
        show|list-unit-files)
            return 1
            ;;
        *)
            return 1
            ;;
    esac
}

journalctl() { return 0; }
ss() { return 0; }

assert_contains "$(get_all_configs)" "$LEGACY_PAQET_DIR/config-dubai.yaml" "legacy /opt/paqet config detected"
assert_eq "paqet-dubai" "$(get_tunnel_service "$LEGACY_PAQET_DIR/config-dubai.yaml")" "legacy paqet-dubai service matched by ExecStart"
status_out="$(check_status 2>/dev/null || true)"
assert_contains "$status_out" "paqet-dubai" "status shows paqet-dubai.service"
assert_contains "$status_out" "$LEGACY_PAQET_DIR/config-dubai.yaml" "status shows legacy config path"
owner_out="$(find_paqet_port_owner "1090" "tcp")"
assert_contains "$owner_out" "paqet-dubai" "port 1090 owner detected as paqet-dubai"
assert_contains "$owner_out" "$LEGACY_PAQET_DIR/config-dubai.yaml" "port 1090 owner reports config"

new_core="$tmp_dir/new-recoba-core"
cat > "$new_core" <<'EOF'
#!/usr/bin/env bash
echo new-core
EOF
chmod +x "$new_core"
rows="paqet-dubai|$LEGACY_PAQET_BIN|$LEGACY_PAQET_DIR/config-dubai.yaml"
install_core_for_active_services "$new_core" "v2.1.5" "v2.1.7" "$rows"
assert_contains "$(cat "$LEGACY_PAQET_BIN")" "new-core" "update-core replaces legacy /opt/paqet/paqet when service uses it"
assert_contains "$(cat "$tmp_dir/restarted-services.log")" "paqet-dubai" "update-core restarts only the owning service"

menu_out="$(render_main_menu)"
assert_contains "$menu_out" "1) Status & diagnostics" "main menu status entry"
assert_contains "$menu_out" "6) Full uninstall / reset this node" "main menu reset entry"

cat > "$SYSTEMD_SYSTEM_DIR/recoba-paqet-tunnel-germany.service" <<EOF
[Service]
ExecStart=$PAQET_BIN run -c $PAQET_DIR/config-germany.yaml
EOF
cat > "$SYSTEMD_SYSTEM_DIR/xray.service" <<'EOF'
[Service]
ExecStart=/usr/bin/xray run
EOF
mkdir -p "$PAQET_DIR" "$LEGACY_PAQET_DIR"
touch "$tmp_dir/placeholder"

targets_out="$(reset_node_targets)"
assert_contains "$targets_out" "path|$LEGACY_PAQET_DIR" "reset target includes legacy /opt/paqet equivalent"
assert_contains "$targets_out" "path|$PAQET_DIR" "reset target includes new install path"
assert_contains "$targets_out" "unit|$SYSTEMD_SYSTEM_DIR/paqet-dubai.service" "reset target includes legacy systemd unit"
assert_contains "$targets_out" "unit|$SYSTEMD_SYSTEM_DIR/recoba-paqet-tunnel-germany.service" "reset target includes new systemd unit"
assert_not_contains "$targets_out" "xray.service" "reset targets exclude xray"
assert_not_contains "$targets_out" "nginx" "reset targets exclude nginx"
assert_not_contains "$targets_out" "x-ui" "reset targets exclude x-ui"

dry_run_out="$(dry_run_full_reset_node)"
assert_contains "$dry_run_out" "DRY-RUN" "reset dry-run announces dry run"
assert_contains "$dry_run_out" "$LEGACY_PAQET_DIR" "reset dry-run shows legacy path"
assert_contains "$dry_run_out" "Will NOT touch" "reset dry-run prints safety exclusions"

check_root() { return 0; }
systemctl() {
    case "${1:-}" in
        cat)
            local svc="${2%.service}"
            [ -f "$SYSTEMD_SYSTEM_DIR/${svc}.service" ] && cat "$SYSTEMD_SYSTEM_DIR/${svc}.service"
            ;;
        is-active)
            return 0
            ;;
        restart|stop|disable|daemon-reload|reset-failed)
            return 0
            ;;
        list-unit-files)
            find "$SYSTEMD_SYSTEM_DIR" -maxdepth 1 -name '*.service' -exec basename {} \;
            ;;
        list-units)
            return 0
            ;;
        show)
            return 1
            ;;
        *)
            return 0
            ;;
    esac
}

help_out="$(handle_cli_args --help)"
assert_contains "$help_out" "--reset-node" "CLI help mentions reset-node"

cli_dry_out="$(handle_cli_args --reset-node --dry-run)"
assert_contains "$cli_dry_out" "DRY-RUN" "CLI reset dry-run works"

mkdir -p "$LEGACY_PAQET_DIR" "$PAQET_DIR"
CLI_ASSUME_YES=false
forced_out="$(handle_cli_args --yes --reset-node)"
assert_contains "$forced_out" "Full reset completed" "CLI --yes reset runs without prompt"
if [ -e "$LEGACY_PAQET_DIR" ] || [ -e "$PAQET_DIR" ]; then
    printf 'FAIL: forced reset did not remove expected install dirs\n' >&2
    exit 1
fi
pass_count=$((pass_count + 1))

printf 'All pure logic tests passed (%s assertions).\n' "$pass_count"
