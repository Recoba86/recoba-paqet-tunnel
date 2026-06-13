# Troubleshooting & Rollback Guide

## Rollback Workflow

If a new deployment or S-flag migration fails validation, roll back to the previous stable state to avoid service degradation.

### 1. Restore Configuration Backup
The installer creates a timestamped backup of your config files before modifying them.
```bash
cp /opt/recoba-paqet-tunnel/config.yaml.bak.<timestamp> /opt/recoba-paqet-tunnel/config.yaml
```

### 2. Remove Path-Specific iptables Persistence
To clean up the systemd helper service and rules for a specific service:
1. Run the installer removal option or remove them manually:
   ```bash
   sudo systemctl disable --now <service>-iptables.service
   sudo rm -f /etc/systemd/system/<service>-iptables.service
   sudo rm -rf /etc/systemd/system/<service>.service.d
   sudo rm -f /opt/recoba-paqet-tunnel/apply-<service>-iptables.sh
   sudo systemctl daemon-reload
   ```
2. Remove the rules from the active kernel by running `-D` commands corresponding to your rules:
   ```bash
   sudo iptables -t raw -D OUTPUT -p tcp -d <server_ip> --dport <port> -j NOTRACK
   sudo iptables -t raw -D PREROUTING -p tcp -s <server_ip> --sport <port> -j NOTRACK
   sudo iptables -t mangle -D OUTPUT -p tcp -d <server_ip> --dport <port> --tcp-flags RST RST -j DROP
   sudo iptables -t mangle -D PREROUTING -p tcp -s <server_ip> --sport <port> --tcp-flags RST RST -j DROP
   ```

---

## Common Failure Points

### 1. Tunnel Fails to Connect / SSL Timeout
* **Symptom**: HTTP proxy works sometimes, but HTTPS times out during SSL handshake.
* **Cause**: Stateful packet inspection is dropping data payload mock-TCP packets. You are likely running legacy `"PA"` flags or server-side suppression is missing.
* **Fix**: Ensure both endpoints are using the S-flag profile, and verify server-side raw mangle rules exist on the server.

### 2. Kernel Sends RST Packets
* **Symptom**: Connection lost logs on client: `paqet logged connection lost, retrying`.
* **Cause**: The server's OS kernel is receiving SYN-ACK/SYN packets and responding with a reset (RST) because it does not recognize the connection.
* **Fix**: Check that the server-side mangle RST drop rule is active:
  ```bash
  sudo iptables -t mangle -S | grep RST
  ```

### 3. Service Ordering Race Conditions
* **Symptom**: After a system reboot, the tunnel service fails to start or gets stuck in a restart loop.
* **Cause**: The tunnel service started before the iptables persistence rules were loaded.
* **Fix**: Ensure that the systemd drop-in `10-iptables.conf` is correctly placed in `/etc/systemd/system/<service>.service.d/` and contains:
  ```ini
  [Unit]
  Wants=<service>-iptables.service
  After=<service>-iptables.service
  ```
  Run `systemctl daemon-reload` and restart the services.
