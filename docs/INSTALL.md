# Installation & Setup Guide

## Overview

The Paqet Raw Packet Tunnel uses standard raw sockets. Setting it up requires configuring:
1. **Server Side**: Listening on a transport port (e.g. `8888`), receiving mock-TCP packets under `"S"` flag, and sup-pressing outgoing kernel resets.
2. **Client Side**: Forwarding local VLESS/socks traffic (e.g. `1090`) to the remote server transport port, sending mock-TCP packets under `"S"` flag, and suppressing outgoing kernel resets.

> [!NOTE]
> The **transport port** (e.g. `8888`, `8889`) is the UDP/raw-TCP port used for tunnel encapsulation on the WAN interface. The **local VLESS/forward port** (e.g. `1090`, `1091`) is the local port bound by the Paqet client to accept incoming proxy connections. Do not mix them up.

## Step-by-Step Installation

### 1. Run the Installer
On both client and server nodes, execute the installer:
```bash
sudo ./install.sh
```

Choose the appropriate menu role:
- **Server B (Abroad)**: For the exit node.
- **Server A (Local)**: For the client/bridge node.

### 2. Configure TCP Flag Profile
During installation or migration, select the **S-flag Production** profile (Choice 1). This ensures configurations are generated with:
- Server: `local_flag: ["S"]`
- Client: `local_flag: ["S"]` and `remote_flag: ["S"]`

### 3. Verification of systemd persistence
The installer automatically generates:
1. Idempotent iptables scripts: `/opt/recoba-paqet-tunnel/apply-<service>-iptables.sh`
2. Oneshot systemd units: `/etc/systemd/system/<service>-iptables.service`
3. Dependency drop-ins: `/etc/systemd/system/<service>.service.d/10-iptables.conf`

Verify that the services are active:
```bash
systemctl status <service>-iptables.service
systemctl status <service>.service
```

---

## Production Configurations Examples

### Dubai Path (Example)
* **Role**: Server
* **Server Transport Port**: `8888`
* **Client Local Listen Port**: `1090` (VLESS)

**Server Config (`/opt/recoba-paqet-tunnel/config.yaml` placeholder)**:
```yaml
role: "server"
listen:
  addr: ":8888"
network:
  interface: "eth0"
  ipv4:
    addr: "10.0.0.1:8888"
    router_mac: "00:11:22:33:44:55"
  tcp:
    local_flag: ["S"]
transport:
  protocol: "kcp"
  conn: 2
  kcp:
    mode: "fast"
    key: "PLACEHOLDER_KEY"
```

**Client Config (`/opt/recoba-paqet-tunnel/config-local-ubuntu.yaml` placeholder)**:
```yaml
role: "client"
forward:
  - listen: "0.0.0.0:1090"
    target: "127.0.0.1:1090"
    protocol: "tcp"
network:
  interface: "eth0"
  ipv4:
    addr: "192.168.1.100:0"
    router_mac: "66:77:88:99:aa:bb"
  tcp:
    local_flag: ["S"]
    remote_flag: ["S"]
server:
  addr: "192.0.2.1:8888"  # Server public IP
transport:
  protocol: "kcp"
  conn: 2
  kcp:
    mode: "fast"
    key: "PLACEHOLDER_KEY"
```

### Sweden Path (Example)
* **Role**: Server
* **Server Transport Port**: `8889`
* **Client Local Listen Ports**: `1091` (VLESS) and alias `1080`

**Server Config (`/opt/recoba-paqet-tunnel/config-sweden.yaml` placeholder)**:
```yaml
role: "server"
listen:
  addr: ":8889"
network:
  interface: "eth0"
  ipv4:
    addr: "10.0.0.2:8889"
    router_mac: "00:11:22:33:44:66"
  tcp:
    local_flag: ["S"]
transport:
  protocol: "kcp"
  conn: 2
  kcp:
    mode: "fast"
    key: "PLACEHOLDER_KEY"
```

**Client Config (`/opt/recoba-paqet-tunnel/config-sweden-client.yaml` placeholder)**:
```yaml
role: "client"
forward:
  - listen: "0.0.0.0:1091"
    target: "127.0.0.1:1091"
    protocol: "tcp"
  - listen: "0.0.0.0:1080"
    target: "127.0.0.1:1091"
    protocol: "tcp"
network:
  interface: "eth0"
  ipv4:
    addr: "192.168.1.100:0"
    router_mac: "66:77:88:99:aa:bb"
  tcp:
    local_flag: ["S"]
    remote_flag: ["S"]
server:
  addr: "198.51.100.2:8889"  # Server public IP
transport:
  protocol: "kcp"
  conn: 2
  kcp:
    mode: "fast"
    key: "PLACEHOLDER_KEY"
```

## Adding a New Exit Node

To add a new exit node path (e.g. Germany):
1. **Server Setup**: Install paqet on Germany server using transport port `8890`. Configure flags as `"S"`.
2. **Client Setup**: Create `/opt/recoba-paqet-tunnel/config-germany-client.yaml` forwarding `1092` to Germany server's IP on port `8890`. Configure flags as `"S"`.
3. **Register Service**: Enable and start a new client systemd service:
   ```bash
   sudo systemctl enable --now paqet-germany-client.service
   ```
   The system will automatically generate `paqet-germany-client-iptables.service` and the drop-in, persisting the rules across reboots.
