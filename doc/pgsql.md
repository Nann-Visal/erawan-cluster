# PostgreSQL 16 + repmgr 5.5 — HA Cluster Configuration Guide
> Auto Failover • Auto Rejoin • Auto Metadata Update • Ubuntu 24.04

## Prerequisites

- PostgreSQL 16 and repmgr 5.5 already installed on both nodes
- Both VMs are fresh from template — clean state

## Cluster Configuration

| Node   | Hostname | IP Address      | Initial Role |
|--------|----------|-----------------|--------------|
| Node 1 | pg01     | 10.38.105.120   | Primary      |
| Node 2 | pg02     | 10.38.105.1     | Standby      |
| Node n | pg0n     | 10.38.105.n     | Standby      |


> ⚠️ Follow phases in order:
> PHASE 1 on BOTH nodes → PHASE 2 on pg01 only → PHASE 3 on pg02 only → PHASE 4 & 5 on BOTH nodes (with node-specific scripts)

---

## PHASE 1 — Base Configuration (Both Nodes)

> ⚠️ Run every step on **BOTH pg01 and pg02**

### Step 1.1 — Create Log Directory
```bash
mkdir -p /var/log/repmgr
chown postgres:postgres /var/log/repmgr
chmod 755 /var/log/repmgr
touch /var/log/repmgr/repmgr.log
chown postgres:postgres /var/log/repmgr/repmgr.log
```

### Step 1.2 — Configure postgresql.conf

```bash
nano /etc/postgresql/16/main/postgresql.conf
```

Find and **comment out** the default listen_addresses line:
```
#listen_addresses = 'localhost'
```

Add all settings at the **very bottom** of the file:
```
# HA Cluster Settings
listen_addresses = '*'
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10
hot_standby = on
wal_log_hints = on
archive_mode = off
shared_preload_libraries = 'repmgr'
```

Verify only ONE active listen_addresses entry exists:
```bash
grep "^listen_addresses" /etc/postgresql/16/main/postgresql.conf
# Must return exactly: listen_addresses = '*'

grep -c "^listen_addresses" /etc/postgresql/16/main/postgresql.conf
# Must return: 1
```

> ⚠️ `listen_addresses = '*'` — CRITICAL. Without it repmgr cannot connect via IP address.
> ⚠️ `shared_preload_libraries = 'repmgr'` — CRITICAL. Without it repmgrd crashes with "unable to write to shared memory".
> ⚠️ `wal_log_hints = on` — CRITICAL. Without it `node rejoin --force-rewind` fails.

### Step 1.3 — Configure pg_hba.conf

```bash
nano /etc/postgresql/16/main/pg_hba.conf
```

Add at the **very bottom**:
```
# Allow all hosts — safe for private/internal networks with no public IP
host    all             all             0.0.0.0/0               scram-sha-256
host    replication     repmgr          0.0.0.0/0               scram-sha-256
host    repmgr          repmgr          0.0.0.0/0               scram-sha-256
```

> ℹ️ `0.0.0.0/0` is acceptable when servers have no public IP and are on a private network.
> For stricter security replace with your subnet e.g. `10.38.105.0/24`.

### Step 1.4 — Enable repmgrd Default Config
```bash
sed -i 's/REPMGRD_ENABLED=no/REPMGRD_ENABLED=yes/' /etc/default/repmgrd
cat /etc/default/repmgrd   # confirm REPMGRD_ENABLED=yes
```

### Step 1.5 — Mask Wrapper Service

> ⚠️ CRITICAL: Always use `postgresql@16-main` — NEVER `postgresql.service`.
> The wrapper shows `active (exited)` and causes repmgrd to lose its connection and crash.

```bash
systemctl mask postgresql.service
```

### Step 1.6 — Configure sudo for postgres User
```bash
echo 'postgres ALL=(ALL) NOPASSWD: /bin/systemctl start postgresql@16-main, /bin/systemctl stop postgresql@16-main, /bin/systemctl restart postgresql@16-main, /bin/systemctl reload postgresql@16-main' \
  > /etc/sudoers.d/postgres-repmgr
chmod 440 /etc/sudoers.d/postgres-repmgr
```

### Step 1.7 — Create .pgpass for postgres User
```bash
cat > /var/lib/postgresql/.pgpass <<EOF
10.38.105.120:5432:repmgr:repmgr:Repmgr#2026
10.38.105.120:5432:replication:repmgr:Repmgr#2026
10.38.105.1:5432:repmgr:repmgr:Repmgr#2026
10.38.105.1:5432:replication:repmgr:Repmgr#2026
*:5432:repmgr:repmgr:Repmgr#2026
*:5432:replication:repmgr:Repmgr#2026
EOF
chmod 600 /var/lib/postgresql/.pgpass
chown postgres:postgres /var/lib/postgresql/.pgpass
```

### Step 1.8 — Create repmgrd systemd Unit
```bash
cat > /etc/systemd/system/repmgrd.service <<EOF
[Unit]
Description=repmgrd - replication manager daemon for PostgreSQL
After=network.target postgresql@16-main.service
Wants=postgresql@16-main.service

[Service]
Type=forking
User=postgres
ExecStart=/usr/bin/repmgrd -f /etc/repmgr.conf --daemonize
ExecReload=/bin/kill -HUP \$MAINPID
PIDFile=/tmp/repmgrd.pid
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
```

### Step 1.9 — Enable Services on Boot
```bash
systemctl enable postgresql@16-main
systemctl enable repmgrd

# Verify
systemctl is-enabled postgresql@16-main   # must show: enabled
systemctl is-enabled repmgrd              # must show: enabled
```

### Step 1.10 — Start PostgreSQL
```bash
systemctl start postgresql@16-main
systemctl status postgresql@16-main | grep Active
# Must show: active (running)

# Confirm listening on all interfaces
ss -tlnp | grep 5432
# Must show: 0.0.0.0:5432
```

---

## PHASE 2 — Configure Primary Node (pg01 only)

> ⚠️ Run on **pg01 (10.38.105.120) ONLY**

### Step 2.1 — Create repmgr User and Database

> ⚠️ pg01 ONLY — pg02 gets this automatically via clone. Never create manually on pg02.

```bash
sudo -u postgres psql <<EOF
CREATE USER repmgr WITH REPLICATION LOGIN PASSWORD 'Repmgr#2026';
CREATE DATABASE repmgr OWNER repmgr;
GRANT ALL PRIVILEGES ON DATABASE repmgr TO repmgr;
ALTER USER repmgr SUPERUSER;
EOF
```

### Step 2.2 — Create /etc/repmgr.conf on pg01
```bash
cat > /etc/repmgr.conf <<EOF
node_id=1
node_name='db-primary'
conninfo='host=10.38.105.120 user=repmgr dbname=repmgr password=Repmgr#2026 connect_timeout=2'
data_directory='/var/lib/postgresql/16/main'
pg_bindir='/usr/lib/postgresql/16/bin'

failover=automatic
promote_command='/usr/bin/repmgr standby promote -f /etc/repmgr.conf'
follow_command='/usr/bin/repmgr standby follow -f /etc/repmgr.conf -W --upstream-node-id=%n'

reconnect_attempts=4
reconnect_interval=5

service_start_command='sudo systemctl start postgresql@16-main'
service_stop_command='sudo systemctl stop postgresql@16-main'
service_restart_command='sudo systemctl restart postgresql@16-main'
service_reload_command='sudo systemctl reload postgresql@16-main'

monitoring_history=yes
monitor_interval_secs=2

log_level=INFO
log_file='/var/log/repmgr/repmgr.log'
EOF
```

### Step 2.3 — Register pg01 as Primary
```bash
sudo -u postgres repmgr -f /etc/repmgr.conf primary register

# Verify
sudo -u postgres repmgr -f /etc/repmgr.conf cluster show
# Must show: node 1 | db-primary | primary | * running
```

### Step 2.4 — Start repmgrd on pg01
```bash
systemctl start repmgrd
systemctl status repmgrd | grep Active
# Must show: active (running)

tail -20 /var/log/repmgr/repmgr.log
# Must show: monitoring cluster primary
```

> ✅ pg01 complete. Proceed to PHASE 3 on pg02.

---

## PHASE 3 — Configure Standby Node (pg02 only)

> ⚠️ Run on **pg02 (10.38.105.1) ONLY**

### Step 3.1 — Create /etc/repmgr.conf on pg02
```bash
cat > /etc/repmgr.conf <<EOF
node_id=2
node_name='db-standby1'
conninfo='host=10.38.105.1 user=repmgr dbname=repmgr password=Repmgr#2026 connect_timeout=2'
data_directory='/var/lib/postgresql/16/main'
pg_bindir='/usr/lib/postgresql/16/bin'

failover=automatic
promote_command='/usr/bin/repmgr standby promote -f /etc/repmgr.conf'
follow_command='/usr/bin/repmgr standby follow -f /etc/repmgr.conf -W --upstream-node-id=%n'

reconnect_attempts=4
reconnect_interval=5

service_start_command='sudo systemctl start postgresql@16-main'
service_stop_command='sudo systemctl stop postgresql@16-main'
service_restart_command='sudo systemctl restart postgresql@16-main'
service_reload_command='sudo systemctl reload postgresql@16-main'

monitoring_history=yes
monitor_interval_secs=2

log_level=INFO
log_file='/var/log/repmgr/repmgr.log'
EOF
```

### Step 3.2 — Stop PostgreSQL on pg02
```bash
systemctl stop postgresql@16-main
```

### Step 3.3 — Clone Standby from Primary
```bash
# Dry run first
sudo -u postgres repmgr -h 10.38.105.120 -U repmgr -d repmgr \
  -f /etc/repmgr.conf standby clone --dry-run

# If dry run is clean, do actual clone
# --force overwrites existing data directory from fresh install
sudo -u postgres repmgr -h 10.38.105.120 -U repmgr -d repmgr \
  -f /etc/repmgr.conf standby clone --force
```

### Step 3.4 — Start PostgreSQL on pg02
```bash
systemctl start postgresql@16-main
systemctl status postgresql@16-main | grep Active
# Must show: active (running)
```

### Step 3.5 — Register pg02 as Standby
```bash
sudo -u postgres repmgr -f /etc/repmgr.conf standby register

# Verify — no warnings should appear
sudo -u postgres repmgr -f /etc/repmgr.conf cluster show
```

Expected:
```
 1 | db-primary  | primary | * running |
 2 | db-standby1 | standby |   running | db-primary
```

### Step 3.6 — Start repmgrd on pg02
```bash
systemctl start repmgrd
systemctl status repmgrd | grep Active
# Must show: active (running)

tail -20 /var/log/repmgr/repmgr.log
# Must show: monitoring connection to upstream node db-primary
```

> ✅ Both nodes configured. Cluster ready for auto-failover.

---

## PHASE 4 — Auto Rejoin + Auto Metadata Update

> ⚠️ Scripts are **different per node**.
> Each node only checks the **other** node — never itself.
> Lock file at `/tmp/pg-auto-rejoin.lock` prevents the script running more than once per boot.
> The lock file is in `/tmp/` so it clears automatically on every reboot.

### How It Works

```
Both nodes reboot simultaneously
        ↓
pg01 waits 20s → checks pg02 → pg02 not ready yet → registers self as PRIMARY ✅
pg02 waits 40s → checks pg01 → pg01 is primary → rejoins as STANDBY ✅

Only pg02 goes down and recovers
        ↓
pg02 waits 40s → checks pg01 → pg01 is primary → rejoins as STANDBY ✅

Only pg01 goes down and recovers
        ↓
pg01 waits 20s → checks pg02 → pg02 is primary → rejoins as STANDBY ✅
```

| Node | Checks Only     | Boot Delay | Wins primary if both reboot together |
|------|-----------------|------------|--------------------------------------|
| pg01 | 10.38.105.1     | 20 seconds | ✅ Yes                               |
| pg02 | 10.38.105.120   | 40 seconds | ❌ No — always defers to pg01        |

---

### Step 4.1 — Create Script on pg01

> ⚠️ Run on **pg01 ONLY**

```bash
cat > /usr/local/bin/pg-auto-rejoin.sh <<'EOF'
#!/bin/bash

LOG="/var/log/repmgr/auto-rejoin.log"
REPMGR_CONF="/etc/repmgr.conf"
REPMGR_PASS="Repmgr#2026"
MY_IP=$(hostname -I | awk '{print $1}')
OTHER_NODES=("10.38.105.1")
NODE_DELAY=20
LOCKFILE="/tmp/pg-auto-rejoin.lock"

# Prevent running more than once per boot
if [ -f "$LOCKFILE" ]; then
  echo "[$(date)] Already ran this boot — skipping." >> $LOG
  exit 0
fi
touch $LOCKFILE

echo "[$(date)] Auto-rejoin started on $MY_IP (delay=${NODE_DELAY}s)..." >> $LOG
sleep $NODE_DELAY
pkill repmgrd 2>/dev/null
sleep 2

# Check other node for active primary
PRIMARY_IP=""
for NODE_IP in "${OTHER_NODES[@]}"; do
  RESULT=$(sudo -u postgres psql \
    "host=$NODE_IP user=repmgr dbname=repmgr password=$REPMGR_PASS connect_timeout=3" \
    -tAc "SELECT pg_is_in_recovery();" 2>/dev/null)
  if [ "$RESULT" = "f" ]; then
    PRIMARY_IP=$NODE_IP
    echo "[$(date)] Found primary at $PRIMARY_IP" >> $LOG
    break
  fi
done

# No other primary found — register self as primary
if [ -z "$PRIMARY_IP" ]; then
  echo "[$(date)] No other primary found — registering self as primary..." >> $LOG
  sudo -u postgres repmgr -f $REPMGR_CONF primary register --force >> $LOG 2>&1
  echo "[$(date)] Metadata updated." >> $LOG
  systemctl start repmgrd
  echo "[$(date)] Done — running as primary." >> $LOG
  exit 0
fi

# Other node is primary — rejoin as standby
echo "[$(date)] Rejoining as standby to $PRIMARY_IP..." >> $LOG
systemctl stop postgresql@16-main
sleep 3

sudo -u postgres repmgr -f $REPMGR_CONF node rejoin \
  -d "host=$PRIMARY_IP user=repmgr dbname=repmgr password=$REPMGR_PASS" \
  --force-rewind >> $LOG 2>&1

if [ $? -eq 0 ]; then
  echo "[$(date)] Rejoin successful — updating metadata..." >> $LOG
  sudo -u postgres repmgr -f $REPMGR_CONF standby register --force >> $LOG 2>&1
  systemctl start postgresql@16-main
  sleep 5
  systemctl start repmgrd
  echo "[$(date)] Done — running as standby." >> $LOG
else
  echo "[$(date)] Rejoin failed — starting PostgreSQL only." >> $LOG
  systemctl start postgresql@16-main
fi
EOF

chmod +x /usr/local/bin/pg-auto-rejoin.sh
```

---

### Step 4.2 — Create Script on pg02

> ⚠️ Run on **pg02 ONLY**

```bash
cat > /usr/local/bin/pg-auto-rejoin.sh <<'EOF'
#!/bin/bash

LOG="/var/log/repmgr/auto-rejoin.log"
REPMGR_CONF="/etc/repmgr.conf"
REPMGR_PASS="Repmgr#2026"
MY_IP=$(hostname -I | awk '{print $1}')
OTHER_NODES=("10.38.105.120")
NODE_DELAY=40
LOCKFILE="/tmp/pg-auto-rejoin.lock"

# Prevent running more than once per boot
if [ -f "$LOCKFILE" ]; then
  echo "[$(date)] Already ran this boot — skipping." >> $LOG
  exit 0
fi
touch $LOCKFILE

echo "[$(date)] Auto-rejoin started on $MY_IP (delay=${NODE_DELAY}s)..." >> $LOG
sleep $NODE_DELAY
pkill repmgrd 2>/dev/null
sleep 2

# Check other node for active primary
PRIMARY_IP=""
for NODE_IP in "${OTHER_NODES[@]}"; do
  RESULT=$(sudo -u postgres psql \
    "host=$NODE_IP user=repmgr dbname=repmgr password=$REPMGR_PASS connect_timeout=3" \
    -tAc "SELECT pg_is_in_recovery();" 2>/dev/null)
  if [ "$RESULT" = "f" ]; then
    PRIMARY_IP=$NODE_IP
    echo "[$(date)] Found primary at $PRIMARY_IP" >> $LOG
    break
  fi
done

# No other primary found — register self as primary
if [ -z "$PRIMARY_IP" ]; then
  echo "[$(date)] No other primary found — registering self as primary..." >> $LOG
  sudo -u postgres repmgr -f $REPMGR_CONF primary register --force >> $LOG 2>&1
  echo "[$(date)] Metadata updated." >> $LOG
  systemctl start repmgrd
  echo "[$(date)] Done — running as primary." >> $LOG
  exit 0
fi

# Other node is primary — rejoin as standby
echo "[$(date)] Rejoining as standby to $PRIMARY_IP..." >> $LOG
systemctl stop postgresql@16-main
sleep 3

sudo -u postgres repmgr -f $REPMGR_CONF node rejoin \
  -d "host=$PRIMARY_IP user=repmgr dbname=repmgr password=$REPMGR_PASS" \
  --force-rewind >> $LOG 2>&1

if [ $? -eq 0 ]; then
  echo "[$(date)] Rejoin successful — updating metadata..." >> $LOG
  sudo -u postgres repmgr -f $REPMGR_CONF standby register --force >> $LOG 2>&1
  systemctl start postgresql@16-main
  sleep 5
  systemctl start repmgrd
  echo "[$(date)] Done — running as standby." >> $LOG
else
  echo "[$(date)] Rejoin failed — starting PostgreSQL only." >> $LOG
  systemctl start postgresql@16-main
fi
EOF

chmod +x /usr/local/bin/pg-auto-rejoin.sh
```

---

### Step 4.3 — Create systemd Service (Both Nodes)
```bash
cat > /etc/systemd/system/pg-auto-rejoin.service <<EOF
[Unit]
Description=PostgreSQL Auto Rejoin after recovery
After=network.target postgresql@16-main.service
Wants=postgresql@16-main.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pg-auto-rejoin.sh
RemainAfterExit=no

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable pg-auto-rejoin.service
systemctl is-enabled pg-auto-rejoin.service   # must show: enabled
```

---

## PHASE 5 — Final Verification

### Step 5.1 — Check All Services Enabled (Both Nodes)
```bash
systemctl is-enabled postgresql@16-main    # enabled
systemctl is-enabled repmgrd               # enabled
systemctl is-enabled pg-auto-rejoin        # enabled
```

### Step 5.2 — Check Cluster Health
```bash
sudo -u postgres repmgr -f /etc/repmgr.conf cluster show
```

Expected — clean with no warnings:
```
 ID | Name        | Role    | Status    | Upstream
----+-------------+---------+-----------+------------
 1  | db-primary  | primary | * running |
 2  | db-standby1 | standby |   running | db-primary
```

### Step 5.3 — Check repmgrd Running on Both Nodes
```bash
ps aux | grep repmgrd
# Must show a postgres process — not just grep
```

### Step 5.4 — Check Replication Active
```bash
# On primary node
sudo -u postgres psql -c "SELECT client_addr, state, sent_lsn, write_lsn, replay_lsn FROM pg_stat_replication;"
```

---

## Test Auto-Failover

> ⚠️ repmgrd must be running on BOTH nodes before testing.

```bash
# Step 1 — Open log on pg02 (keep open)
tail -f /var/log/repmgr/repmgr.log

# Step 2 — Stop primary on pg01
systemctl stop postgresql@16-main

# Step 3 — Watch pg02 promote itself (~20 seconds)
```

Expected pg02 log:
```
[WARNING] unable to ping upstream node db-primary (ID: 1)
[INFO]    checking state of node 1, attempt 1 of 4
[INFO]    checking state of node 1, attempt 2 of 4
[INFO]    checking state of node 1, attempt 3 of 4
[INFO]    checking state of node 1, attempt 4 of 4
[NOTICE]  this node is the only available candidate and will now promote itself
[NOTICE]  STANDBY PROMOTE successful
[NOTICE]  node 2 promoted to primary
[NOTICE]  monitoring cluster primary db-standby1 (ID: 2)
```

---

## Test Auto-Rejoin After Recovery

```bash
# Step 1 — On recovered node, start PostgreSQL
systemctl start postgresql@16-main

# Step 2 — Trigger auto-rejoin (runs automatically on reboot)
systemctl start pg-auto-rejoin.service

# Step 3 — Watch log
tail -f /var/log/repmgr/auto-rejoin.log
```

Expected log:
```
Auto-rejoin started on <IP> (delay=Xs)...
Found primary at <PRIMARY_IP>
Rejoining as standby to <PRIMARY_IP>...
Rejoin successful — updating metadata...
Done — running as standby.
```

---

## Quick Reference

### Cluster Health
```bash
sudo -u postgres repmgr -f /etc/repmgr.conf cluster show
ps aux | grep repmgrd
tail -50 /var/log/repmgr/repmgr.log
tail -50 /var/log/repmgr/auto-rejoin.log
```

### Service Commands
```bash
# PostgreSQL — always use versioned name, NEVER postgresql.service
systemctl start postgresql@16-main
systemctl stop postgresql@16-main
systemctl restart postgresql@16-main
systemctl status postgresql@16-main

# repmgrd
systemctl start repmgrd
systemctl stop repmgrd
systemctl restart repmgrd
systemctl status repmgrd
```

### Manual Failover
```bash
# On standby node
sudo -u postgres repmgr -f /etc/repmgr.conf standby promote
sudo -u postgres repmgr -f /etc/repmgr.conf primary register --force
```

### Manual Rejoin
```bash
# Stop first — rejoin cannot run on a running node
systemctl stop postgresql@16-main
pkill repmgrd

sudo -u postgres repmgr -f /etc/repmgr.conf node rejoin \
  -d 'host=<PRIMARY_IP> user=repmgr dbname=repmgr password=Repmgr#2026' \
  --force-rewind --verbose

systemctl start postgresql@16-main
systemctl start repmgrd
```

### Trigger Auto-Rejoin Manually
```bash
# Remove lock file first if script already ran this boot
rm -f /tmp/pg-auto-rejoin.lock
systemctl start pg-auto-rejoin.service
tail -f /var/log/repmgr/auto-rejoin.log
```

---

## Common Errors & Fixes

| Error | Cause | Fix |
|-------|-------|-----|
| `Connection refused` on IP | `listen_addresses` not set to `*` | Comment out `listen_addresses = 'localhost'`, add `listen_addresses = '*'` at bottom, verify only 1 active entry, restart |
| `unable to write to shared memory` | Missing `shared_preload_libraries = 'repmgr'` | Add to postgresql.conf and restart |
| `active (exited)` on repmgrd | repmgrd crashed | Check `/var/log/repmgr/repmgr.log` |
| `active (exited)` on postgresql | Wrong service name | Use `postgresql@16-main` not `postgresql` |
| `cannot be run as root` | Running repmgrd as root | Use `sudo -u postgres repmgrd ...` |
| `upstream node must be running` | repmgrd started after primary already down | Start primary first, then start repmgrd |
| `wal_log_hints is off` | Missing config for pg_rewind | Add `wal_log_hints = on` to postgresql.conf and restart |
| `node rejoin cannot run on running node` | PostgreSQL still running | `systemctl stop postgresql@16-main` and `pkill repmgrd` first |
| `password authentication failed` | .pgpass missing or wrong permissions | Recreate `/var/lib/postgresql/.pgpass` with `chmod 600` |
| `role repmgr does not exist` on standby | pg02 not cloned from primary | Stop pg02, run `standby clone --force` |
| duplicate repmgrd processes | Manual start while systemd also running | `pkill repmgrd` then `systemctl start repmgrd` |
| split-brain after recovery | Both nodes became primary | Stop the lower-timeline node, run `node rejoin --force-rewind` to current primary |
| auto-rejoin script runs multiple times | No lock file | Script now has lock file — clears automatically on reboot |
| `already registered` error | Node registered twice | Use `primary register --force` or `standby register --force` |

---

## Credentials Summary

| User     | Password               | Database | Purpose     |
|----------|------------------------|----------|-------------|
| postgres | NewStrongPassword#2026 | postgres | Superuser   |
| repmgr   | Repmgr#2026            | repmgr   | Replication |