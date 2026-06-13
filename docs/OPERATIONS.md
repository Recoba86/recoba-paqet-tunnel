# Operations & Durability Guide

## Reboot Durability Workflow

Reboot durability ensures that all network-level raw socket bypass rules and tunnel stream handlers recover automatically after a node reboots, without manual intervention.

### Server Node Reboot Durability Checklist
To verify a server node reboot durability:
1. Ensure the systemd iptables service is active and enabled:
   ```bash
   systemctl is-enabled recoba-paqet-tunnel-iptables.service
   ```
2. Trigger reboot:
   ```bash
   sudo reboot
   ```
3. Once back online, check the service status and logs:
   ```bash
   systemctl status recoba-paqet-tunnel-iptables.service
   systemctl status recoba-paqet-tunnel.service
   ```
4. Verify that the iptables rules exist on the interface:
   ```bash
   sudo iptables -t raw -S | grep -E "8888|8889"
   sudo iptables -t mangle -S | grep -E "8888|8889"
   ```
5. Confirm client connection logs in `/var/log/syslog` or journalctl:
   ```bash
   journalctl -u recoba-paqet-tunnel -b --no-pager
   ```

### Client Node Reboot Durability Checklist
To verify the client node reboot durability:
1. Confirm the iptables helper services for all tunnels are active and enabled:
   ```bash
   systemctl is-enabled paqet-local-ubuntu-iptables.service
   systemctl is-enabled paqet-sweden-client-iptables.service
   ```
2. Reboot the client node.
3. Verify all tunnel services recover:
   ```bash
   systemctl is-active paqet-local-ubuntu paqet-sweden-client
   ```
4. Confirm listeners are active:
   ```bash
   ss -tlnp | grep -E "1090|1091|1080"
   ```

---

## Validation Workflow

Before considering any tunnel fully production-ready, perform the following validation workflow.

### 1. Uptime and Process Check
Ensure the service uptime is clean and no rapid restarts are occurring.
```bash
systemctl status paqet-local-ubuntu
```

### 2. Idempotency Check
Execute the generated iptables script twice to ensure it does not create duplicate entries in raw or mangle tables.
```bash
sudo /opt/recoba-paqet-tunnel/apply-paqet-local-ubuntu-iptables.sh
sudo iptables -t raw -S | grep NOTRACK
```

### 3. Traffic Carriage Verification
Run a validation loop through the local proxy using curl.

**HTTP Test**:
```bash
curl -s -o /dev/null -w "%{http_code}\n" -x http://127.0.0.1:39080 http://example.com
```

**HTTPS Test**:
```bash
curl -s -o /dev/null -w "%{http_code}\n" -x http://127.0.0.1:39080 https://www.google.com
```

**20-Request Loop (Reliability Check)**:
```bash
for i in {1..20}; do
  curl -s -o /dev/null -w "%{http_code} " -x http://127.0.0.1:39080 https://www.google.com
done
echo ""
```
The output should report `200` exactly 20 times, confirming 100% carriage stability.
