# HA Cluster — Full Step-by-Step Implementation Guide
## Ubuntu 24.04 | MySQL 8.0 | PostgreSQL 16

**Node Layout**

| Node | IP | Role |
|---|---|---|
| Node 1 | 192.168.1.10 | HAProxy |
| Node 2 | 192.168.1.11 | Replication Manager + etcd |
| Node 3 | 192.168.1.12 | DB Primary (MySQL + PostgreSQL) |
| Node 4 | 192.168.1.13 | DB Standby (MySQL + PostgreSQL) |

**Implementation Order**
1. Node 3 & 4 — MySQL 8.0 Primary/Standby
2. Node 3 & 4 — PostgreSQL 16 + Patroni
3. Node 2 — Replication Manager + etcd
4. Node 1 — HAProxy

---

# STACK 1 — MySQL 8.0 Primary/Standby
## Nodes: 3 & 4

---

## Step 1.1 — Install MySQL 8.0 on Node 3 AND Node 4

```bash
# Run on BOTH Node 3 and Node 4
apt update && apt upgrade -y

# Install MySQL 8.0
apt install -y mysql-server mysql-client

# Verify version
mysql --version
# Expected: mysql  Ver 8.0.x

# Enable and start
systemctl enable mysql
systemctl start mysql
systemctl status mysql
```

---

## Step 1.2 — Configure MySQL on Node 3 (Primary)

```bash
# Edit MySQL config on Node 3
nano /etc/mysql/mysql.conf.d/mysqld.cnf
```

Add/update these lines:

```ini
[mysqld]
# Basic settings
bind-address            = 0.0.0.0
server-id               = 1                  # MUST be unique per node
port                    = 3306

# Binary logging (required for replication)
log_bin                 = /var/log/mysql/mysql-bin.log
binlog_expire_logs_seconds = 604800          # 7 days retention
max_binlog_size         = 100M
binlog_format           = ROW               # Required for MySQL 8.0

# GTID (required for replication-manager autorejoin)
gtid_mode               = ON
enforce_gtid_consistency = ON

# Replication settings
log_replica_updates     = ON
replica_preserve_commit_order = ON

# Performance
innodb_buffer_pool_size = 1G               # adjust to 70% of RAM
innodb_flush_log_at_trx_commit = 1
sync_binlog             = 1
```

```bash
# Restart MySQL
systemctl restart mysql
```

---

## Step 1.3 — Configure MySQL on Node 4 (Standby)

```bash
# Edit MySQL config on Node 4
nano /etc/mysql/mysql.conf.d/mysqld.cnf
```

```ini
[mysqld]
# Basic settings
bind-address            = 0.0.0.0
server-id               = 2                  # MUST be different from Node 3
port                    = 3306

# Binary logging
log_bin                 = /var/log/mysql/mysql-bin.log
binlog_expire_logs_seconds = 604800
max_binlog_size         = 100M
binlog_format           = ROW

# GTID
gtid_mode               = ON
enforce_gtid_consistency = ON

# Replication
log_replica_updates     = ON
replica_preserve_commit_order = ON
read_only               = ON                 # Standby is read-only
super_read_only         = ON

# Performance
innodb_buffer_pool_size = 1G
innodb_flush_log_at_trx_commit = 1
sync_binlog             = 1
```

```bash
# Restart MySQL
systemctl restart mysql
```

---

## Step 1.4 — Secure MySQL on Node 3 (Primary)

```bash
# Run secure installation
mysql_secure_installation
# Set root password, remove anonymous users, disallow remote root, remove test db

# Login as root
mysql -u root -p
```

```sql
-- Create replication user
CREATE USER 'replicator'@'%' IDENTIFIED WITH mysql_native_password BY 'ReplPass123!';
GRANT REPLICATION SLAVE ON *.* TO 'replicator'@'%';

-- Create replication-manager monitoring user
CREATE USER 'repmgr'@'%' IDENTIFIED WITH mysql_native_password BY 'RepmgrPass123!';
GRANT SUPER, REPLICATION CLIENT, REPLICATION SLAVE,
      RELOAD, PROCESS, SHOW DATABASES,
      EVENT, TRIGGER ON *.* TO 'repmgr'@'%';

-- Create app user
CREATE USER 'appuser'@'%' IDENTIFIED WITH mysql_native_password BY 'AppPass123!';
GRANT SELECT, INSERT, UPDATE, DELETE ON *.* TO 'appuser'@'%';

FLUSH PRIVILEGES;

-- Verify GTID is ON
SHOW VARIABLES LIKE 'gtid_mode';
-- Expected: gtid_mode | ON
```

---

## Step 1.5 — Set Up Replication from Node 4 to Node 3

```bash
# On Node 3 — get binary log position
mysql -u root -p
```

```sql
-- On Node 3 (Primary)
SHOW MASTER STATUS\G
-- Note: File and Position values
-- With GTID enabled, position is less important but verify gtid_executed
SHOW VARIABLES LIKE 'gtid_executed';
```

```bash
# On Node 4 — connect and configure replication
mysql -u root -p
```

```sql
-- On Node 4 (Standby)
-- Stop replica if running
STOP REPLICA;
RESET REPLICA ALL;

-- Point to primary
CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='192.168.1.12',
  SOURCE_PORT=3306,
  SOURCE_USER='replicator',
  SOURCE_PASSWORD='ReplPass123!',
  SOURCE_AUTO_POSITION=1;         -- Uses GTID auto positioning

-- Start replication
START REPLICA;

-- Verify replication is running
SHOW REPLICA STATUS\G
-- Check:
--   Replica_IO_Running: Yes
--   Replica_SQL_Running: Yes
--   Seconds_Behind_Source: 0
```

---

## Step 1.6 — Verify MySQL Replication

```bash
# On Node 3 — create test database
mysql -u root -p -e "CREATE DATABASE repl_test;"

# On Node 4 — verify it replicated
mysql -u root -p -e "SHOW DATABASES;" | grep repl_test
# Expected: repl_test

# Clean up test
mysql -u root -p -e "DROP DATABASE repl_test;"
```

### ✅ MySQL Stack Complete — Checklist

```
□ MySQL 8.0 installed on Node 3 and Node 4
□ server-id = 1 on Node 3, server-id = 2 on Node 4
□ GTID enabled on both nodes
□ replicator user created
□ repmgr user created
□ appuser created
□ Replication running (IO: Yes, SQL: Yes)
□ Test database replicated successfully
```

---
---

# STACK 2 — PostgreSQL 16 + Patroni
## Nodes: 3 & 4 (and etcd on Node 2)

---

## Step 2.1 — Install PostgreSQL 16 on Node 3 AND Node 4

```bash
# Run on BOTH Node 3 and Node 4

# Add PostgreSQL official repo
apt install -y curl ca-certificates
install -d /usr/share/postgresql-common/pgdg
curl -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc --fail \
  https://www.postgresql.org/media/keys/ACCC4CF8.asc

sh -c 'echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
  https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
  > /etc/apt/sources.list.d/pgdg.list'

apt update
apt install -y postgresql-16 postgresql-client-16

# IMPORTANT: Stop and disable — Patroni will manage PostgreSQL
systemctl stop postgresql
systemctl disable postgresql

# Verify install
psql --version
# Expected: psql (PostgreSQL) 16.x
```

---

## Step 2.2 — Install Patroni on Node 3 AND Node 4

```bash
# Run on BOTH Node 3 and Node 4
apt install -y python3-pip python3-dev libpq-dev

# Install Patroni with etcd support
pip3 install patroni[etcd] psycopg2-binary

# Verify
patroni --version
# Expected: patroni 3.x.x
```

---

## Step 2.3 — Install etcd on Node 2

```bash
# Run on Node 2 ONLY
apt install -y etcd

# Configure etcd
nano /etc/default/etcd
```

```ini
ETCD_NAME="etcd-node2"
ETCD_DATA_DIR="/var/lib/etcd"
ETCD_LISTEN_PEER_URLS="http://192.168.1.11:2380"
ETCD_LISTEN_CLIENT_URLS="http://192.168.1.11:2379,http://127.0.0.1:2379"
ETCD_INITIAL_ADVERTISE_PEER_URLS="http://192.168.1.11:2380"
ETCD_ADVERTISE_CLIENT_URLS="http://192.168.1.11:2379"
ETCD_INITIAL_CLUSTER="etcd-node2=http://192.168.1.11:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="pg-etcd-cluster"
```

```bash
# Start etcd
systemctl enable etcd
systemctl start etcd

# Verify etcd is running
etcdctl endpoint health
# Expected: 127.0.0.1:2379 is healthy
```

---

## Step 2.4 — Configure Patroni on Node 3 (Primary)

```bash
# On Node 3
mkdir -p /etc/patroni
nano /etc/patroni/patroni.yml
```

```yaml
scope: pg-cluster
namespace: /db/
name: pg-node3

restapi:
  listen: 192.168.1.12:8008
  connect_address: 192.168.1.12:8008

etcd:
  hosts: 192.168.1.11:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576    # 1MB max lag before failover
  initdb:
    - encoding: UTF8
    - data-checksums
  postgresql:
    use_pg_rewind: true
    use_slots: true
    parameters:
      wal_level: replica
      hot_standby: "on"
      max_wal_senders: 5
      max_replication_slots: 5
      wal_log_hints: "on"              # Required for pg_rewind
      archive_mode: "off"

postgresql:
  listen: 192.168.1.12:5432
  connect_address: 192.168.1.12:5432
  data_dir: /var/lib/postgresql/16/main
  bin_dir: /usr/lib/postgresql/16/bin
  pgpass: /tmp/pgpass0
  authentication:
    replication:
      username: replicator
      password: ReplPass123!
    superuser:
      username: postgres
      password: SuperPass123!
    rewind:
      username: rewind_user
      password: RewindPass123!
  callbacks:
    on_role_change: /etc/patroni/haproxy_sync.sh

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
```

---

## Step 2.5 — Configure Patroni on Node 4 (Standby)

```bash
# On Node 4
mkdir -p /etc/patroni
nano /etc/patroni/patroni.yml
```

```yaml
scope: pg-cluster
namespace: /db/
name: pg-node4                          # different from Node 3

restapi:
  listen: 192.168.1.13:8008             # Node 4 IP
  connect_address: 192.168.1.13:8008

etcd:
  hosts: 192.168.1.11:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
  initdb:
    - encoding: UTF8
    - data-checksums
  postgresql:
    use_pg_rewind: true
    use_slots: true
    parameters:
      wal_level: replica
      hot_standby: "on"
      max_wal_senders: 5
      max_replication_slots: 5
      wal_log_hints: "on"
      archive_mode: "off"

postgresql:
  listen: 192.168.1.13:5432             # Node 4 IP
  connect_address: 192.168.1.13:5432
  data_dir: /var/lib/postgresql/16/main
  bin_dir: /usr/lib/postgresql/16/bin
  pgpass: /tmp/pgpass0
  authentication:
    replication:
      username: replicator
      password: ReplPass123!
    superuser:
      username: postgres
      password: SuperPass123!
    rewind:
      username: rewind_user
      password: RewindPass123!
  callbacks:
    on_role_change: /etc/patroni/haproxy_sync.sh

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
```

---

## Step 2.6 — Create HAProxy Sync Callback Script (Node 3 & 4)

```bash
# Run on BOTH Node 3 and Node 4
nano /etc/patroni/haproxy_sync.sh
```

```bash
#!/bin/bash
ROLE=$1
MY_IP=$(hostname -I | awk '{print $1}')
HAPROXY_HOST="192.168.1.10"
HAPROXY_STATS_PORT="9999"
LOGFILE="/var/log/patroni-haproxy-sync.log"

echo "$(date): Role changed to $ROLE for $MY_IP" >> $LOGFILE

# Use socat if HAProxy stats socket is available locally
# Otherwise signal HAProxy via HTTP stats interface
if [ "$ROLE" = "master" ]; then
  echo "$(date): $MY_IP promoted to PRIMARY" >> $LOGFILE
else
  echo "$(date): $MY_IP is now REPLICA" >> $LOGFILE
fi
```

```bash
chmod +x /etc/patroni/haproxy_sync.sh
```

---

## Step 2.7 — Create Patroni systemd Service (Node 3 & 4)

```bash
# Run on BOTH Node 3 and Node 4
nano /etc/systemd/system/patroni.service
```

```ini
[Unit]
Description=Patroni - PostgreSQL HA
After=network.target

[Service]
Type=simple
User=postgres
Group=postgres
ExecStart=/usr/local/bin/patroni /etc/patroni/patroni.yml
ExecReload=/bin/kill -s HUP $MAINPID
KillMode=process
TimeoutSec=30
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

```bash
# Fix permissions
chown -R postgres:postgres /etc/patroni
chmod 700 /var/lib/postgresql/16/main

systemctl daemon-reload
systemctl enable patroni
```

---

## Step 2.8 — Start Patroni

```bash
# Start Node 3 FIRST (becomes primary)
# On Node 3:
systemctl start patroni
journalctl -u patroni -f    # watch logs

# Wait until Node 3 shows as Leader, then start Node 4
# On Node 4:
systemctl start patroni
journalctl -u patroni -f    # watch logs
```

---

## Step 2.9 — Verify PostgreSQL + Patroni

```bash
# On Node 2 (or any node with patronictl)
pip3 install patroni[etcd]

patronictl -c /etc/patroni/patroni.yml list
```

Expected output:
```
+ Cluster: pg-cluster --------+----+-----------+
| Member    | Host            | Role    | State   |
+-----------+-----------------+---------+---------+
| pg-node3  | 192.168.1.12    | Leader  | running |
| pg-node4  | 192.168.1.13    | Replica | running |
+-----------+-----------------+---------+---------+
```

```bash
# Test PostgreSQL connection
psql -h 192.168.1.12 -U postgres -c "SELECT version();"

# Create rewind user (required for pg_rewind on failover)
psql -h 192.168.1.12 -U postgres -c "
  CREATE USER rewind_user WITH REPLICATION PASSWORD 'RewindPass123!';
  GRANT EXECUTE ON function pg_catalog.pg_ls_dir(text, boolean, boolean) TO rewind_user;
  GRANT EXECUTE ON function pg_catalog.pg_stat_file(text, boolean) TO rewind_user;
  GRANT EXECUTE ON function pg_catalog.pg_read_binary_file(text) TO rewind_user;
  GRANT EXECUTE ON function pg_catalog.pg_read_binary_file(text, bigint, bigint, boolean) TO rewind_user;
"

# Create app user
psql -h 192.168.1.12 -U postgres -c "
  CREATE USER appuser WITH PASSWORD 'AppPass123!';
  GRANT CONNECT ON DATABASE postgres TO appuser;
"
```

### ✅ PostgreSQL Stack Complete — Checklist

```
□ PostgreSQL 16 installed on Node 3 and Node 4
□ PostgreSQL service disabled (Patroni manages it)
□ etcd running on Node 2
□ Patroni config created on Node 3 and Node 4
□ Patroni service running on both nodes
□ patronictl list shows Leader + Replica
□ rewind_user created
□ appuser created
```

---
---

# STACK 3 — Replication Manager + etcd
## Node 2

---

## Step 3.1 — Install signal18 Replication Manager on Node 2

```bash
# On Node 2
# Download latest replication-manager
REPMGR_VERSION="2.2.29"    # check latest at github.com/signal18/replication-manager
wget https://github.com/signal18/replication-manager/releases/download/v${REPMGR_VERSION}/replication-manager-osc_${REPMGR_VERSION}_amd64.deb \
  -O /tmp/replication-manager.deb

dpkg -i /tmp/replication-manager.deb

# Or install binary directly
wget https://github.com/signal18/replication-manager/releases/latest/download/replication-manager-osc \
  -O /usr/bin/replication-manager
chmod +x /usr/bin/replication-manager

# Verify
replication-manager --version
```

---

## Step 3.2 — Create Config Directory

```bash
# On Node 2
mkdir -p /etc/replication-manager
mkdir -p /var/lib/replication-manager
mkdir -p /var/log/replication-manager
```

---

## Step 3.3 — Configure Replication Manager

```bash
nano /etc/replication-manager/config.toml
```

```toml
# ==============================================
# signal18 Replication Manager Configuration
# Ubuntu 24.04 | MySQL 8.0
# ==============================================

[default]
title = "mysql-cluster"

# --- DB Nodes ---
hosts = "192.168.1.12:3306,192.168.1.13:3306"
user = "repmgr:RepmgrPass123!"
replication-user = "replicator:ReplPass123!"
replication-credential = "replicator:ReplPass123!"

# --- Failover ---
failover-mode = "automatic"
failover-limit = 5                      # max auto failovers before pausing
failover-time-limit = 30                # seconds to confirm primary is dead
failover-at-sync = true                 # only failover if standby is in sync
failover-restart-unsafe = false

# --- Auto Rejoin (old primary recovery) ---
autorejoin = true
autorejoin-flashback = true             # fast resync via binlog
autorejoin-backup-binlog = true
autorejoin-mysqldump = false            # fallback: full dump (slower)
autorejoin-slave-positional-heartbeat = true

# --- GTID ---
replication-use-gtid = "slave_pos"      # use GTID for MySQL 8.0

# --- HAProxy on Node 1 handles routing automatically ---
# No proxy integration needed in replication-manager

# --- Monitoring ---
monitoring-save-config = true
monitoring-sharedir = "/var/lib/replication-manager"

# --- Logging ---
log-file = "/var/log/replication-manager/replication-manager.log"
log-level = 1

# --- Web UI ---
http-server = true
http-bind-address = "0.0.0.0"
http-port = "10001"
http-auth = true
http-bootstrap-button = true
```

---

## Step 3.4 — Create systemd Service for Replication Manager

```bash
nano /etc/systemd/system/replication-manager.service
```

```ini
[Unit]
Description=signal18 Replication Manager
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/bin/replication-manager monitor \
  --config=/etc/replication-manager/config.toml
ExecReload=/bin/kill -s HUP $MAINPID
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable replication-manager
systemctl start replication-manager

# Watch logs
journalctl -u replication-manager -f
```

---

## Step 3.5 — Verify Replication Manager

```bash
# Check topology
replication-manager-cli --cluster=mysql-cluster topology

# Expected output:
# +------------------+--------+---------------+----------+
# | Host             | Status | Role          | GTID     |
# +------------------+--------+---------------+----------+
# | 192.168.1.12     | OK     | master        | ...      |
# | 192.168.1.13     | OK     | slave         | ...      |
# +------------------+--------+---------------+----------+

# Access Web UI
# http://192.168.1.11:10001
```

---

## Step 3.6 — How Auto Rejoin Works (After Old Primary Recovers)

This is handled automatically by `autorejoin = true` in config.toml. Here is exactly what happens:

### Failover Flow (Node 3 crashes)
```
Node 3 MySQL crashes
        │
        ▼
replication-manager detects failure (~3s)
        │
        ▼
Promotes Node 4 → new primary
SET read_only = OFF on Node 4
        │
        ▼
HAProxy health check detects Node 3 down
Routes all traffic to Node 4 ✅
```

### Rejoin Flow (Node 3 comes back)
```
Node 3 MySQL comes back online
        │
        ▼
replication-manager detects Node 3 is alive
        │
        ▼
Compares GTID position:
  ├── Node 3 is BEHIND Node 4?
  │     → flashback via binlog (fast, seconds)
  └── Node 3 is AHEAD of Node 4? (had extra transactions)
        → mysqldump full resync (slower, safe)
        │
        ▼
replication-manager reconfigures Node 3:
  SET read_only = ON
  CHANGE REPLICATION SOURCE TO master_host='Node4'
  START REPLICA
        │
        ▼
Node 3 rejoins as standby replica ✅
HAProxy keeps routing to Node 4 (still primary)
```

### To verify rejoin after Node 3 recovers:
```bash
# Watch replication-manager detect and rejoin Node 3
replication-manager-cli --cluster=mysql-cluster topology

# Expected after rejoin:
# | 192.168.1.13     | OK     | master  |   ← Node 4 still primary
# | 192.168.1.12     | OK     | slave   |   ← Node 3 rejoined as standby

# Check replication is running on Node 3
mysql -h 192.168.1.12 -u root -p -e "SHOW REPLICA STATUS\G" | grep -E "Running|Behind"
# Expected:
#   Replica_IO_Running: Yes
#   Replica_SQL_Running: Yes
#   Seconds_Behind_Source: 0
```

### Optional — Switchback to Node 3 as Primary (graceful)
```bash
# Only do this if you want Node 3 to be primary again
# This is a graceful zero-downtime switchover
replication-manager-cli --cluster=mysql-cluster switchover

# After switchover:
# | 192.168.1.12     | OK     | master  |   ← Node 3 primary again
# | 192.168.1.13     | OK     | slave   |   ← Node 4 back to standby
```

### ✅ Replication Manager Complete — Checklist

```
□ replication-manager binary installed on Node 2
□ config.toml created with correct IPs and credentials
□ autorejoin = true confirmed in config
□ systemd service running
□ topology shows Node 3 as master, Node 4 as slave
□ Web UI accessible at http://192.168.1.11:10001
□ Test failover: stop Node 3 MySQL → Node 4 promotes
□ Test rejoin: start Node 3 MySQL → rejoins as slave
```

---
---

# STACK 4 — HAProxy
## Node 1

---

## Step 4.1 — Install HAProxy on Node 1

```bash
# On Node 1
apt update
apt install -y haproxy

# Verify version (need 2.x+)
haproxy -v
# Expected: HAProxy version 2.x.x

systemctl enable haproxy
```

---

## Step 4.2 — Configure HAProxy

```bash
# Backup default config
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak

nano /etc/haproxy/haproxy.cfg
```

```cfg
#---------------------------------------------------------------------
# Global settings
#---------------------------------------------------------------------
global
    log /dev/log    local0
    log /dev/log    local1 notice
    chroot /var/lib/haproxy
    stats socket /run/haproxy/admin.sock mode 660 level admin expose-fd listeners
    stats timeout 30s
    user haproxy
    group haproxy
    daemon
    maxconn 4096

#---------------------------------------------------------------------
# Default settings
#---------------------------------------------------------------------
defaults
    log     global
    mode    tcp
    option  tcplog
    option  dontlognull
    timeout connect 5s
    timeout client  30s
    timeout server  30s
    retries 3

#---------------------------------------------------------------------
# HAProxy Stats UI
#---------------------------------------------------------------------
listen stats
    bind *:9999
    mode http
    stats enable
    stats uri /stats
    stats refresh 5s
    stats auth admin:HaproxyAdmin123!

#---------------------------------------------------------------------
# MySQL — primary only, read and write (port 3306)
#---------------------------------------------------------------------
frontend mysql_front
    bind *:3306
    mode tcp
    default_backend mysql_primary

backend mysql_primary
    mode tcp
    option tcp-check
    server mysql-node3 192.168.1.12:3306 check inter 2s rise 2 fall 3
    server mysql-node4 192.168.1.13:3306 check inter 2s rise 2 fall 3 backup

#---------------------------------------------------------------------
# PostgreSQL — primary only, read and write (port 5432)
#---------------------------------------------------------------------
frontend pgsql_front
    bind *:5432
    mode tcp
    default_backend pg_primary

backend pg_primary
    mode tcp
    option tcp-check
    tcp-check connect
    server pg-node3 192.168.1.12:5432 check inter 2s rise 2 fall 3
    server pg-node4 192.168.1.13:5432 check inter 2s rise 2 fall 3 backup
```

```bash
# Validate config
haproxy -c -f /etc/haproxy/haproxy.cfg
# Expected: Configuration file is valid

systemctl start haproxy
systemctl status haproxy
```

---

## Step 4.3 — Verify Full Stack

```bash
# --- Test MySQL (port 3306 → primary only) ---
mysql -h 192.168.1.10 -P 3306 -u appuser -p'AppPass123!' \
  -e "SELECT @@hostname, @@read_only;"
# Expected: node3, 0 (primary)

# --- Test PostgreSQL (port 5432 → primary only) ---
psql -h 192.168.1.10 -p 5432 -U postgres \
  -c "SELECT inet_server_addr(), pg_is_in_recovery();"
# Expected: 192.168.1.12, f (primary, not in recovery)

# --- HAProxy stats ---
# Open in browser: http://192.168.1.10:9999/stats

# --- Replication Manager topology ---
replication-manager-cli --cluster=mysql-cluster topology

# --- Patroni cluster status ---
patronictl -c /etc/patroni/patroni.yml list
```

### ✅ HAProxy Complete — Checklist

```
□ HAProxy installed and running on Node 1
□ HAProxy config validated (haproxy -c)
□ MySQL read/write via port 3306 working (primary only)
□ PostgreSQL read/write via port 5432 working (primary only)
□ HAProxy stats accessible at :9999/stats
```

---
---

# FAILOVER TESTS — Run Before Go-Live

## Test 1 — MySQL Primary Failure

```bash
# Simulate Node 3 MySQL crash
systemctl stop mysql          # on Node 3

# Watch replication-manager promote Node 4 (~3s)
replication-manager-cli --cluster=mysql-cluster topology

# Verify HAProxy rerouted to Node 4
mysql -h 192.168.1.10 -P 3306 -u appuser -p'AppPass123!' \
  -e "SELECT @@hostname, @@read_only;"
# Expected: node4, 0 (Node4 is now primary)
# Restore Node 3
systemctl start mysql         # on Node 3
# Watch autorejoin — Node 3 should rejoin as slave (~30s)
replication-manager-cli --cluster=mysql-cluster topology
```

## Test 2 — PostgreSQL Primary Failure

```bash
# Simulate Node 3 PostgreSQL crash
patronictl -c /etc/patroni/patroni.yml failover pg-cluster --master pg-node3 --force

# Watch Node 4 become leader
patronictl -c /etc/patroni/patroni.yml list

# Node 3 auto-rejoins as replica via pg_rewind
# Watch logs
journalctl -u patroni -f      # on Node 3
```

## Test 3 — Manual Switchover (graceful, zero downtime)

```bash
# MySQL graceful switchover
replication-manager-cli --cluster=mysql-cluster switchover

# PostgreSQL graceful switchover
patronictl -c /etc/patroni/patroni.yml switchover pg-cluster
```

---

# Final Summary

| Component | Node | Status Check |
|---|---|---|
| MySQL Primary | Node 3 | `mysql -h 192.168.1.12 -e "SHOW MASTER STATUS"` |
| MySQL Standby | Node 4 | `mysql -h 192.168.1.13 -e "SHOW REPLICA STATUS\G"` |
| PostgreSQL Cluster | Node 3+4 | `patronictl -c /etc/patroni/patroni.yml list` |
| Replication Manager | Node 2 | `http://192.168.1.11:10001` |
| etcd | Node 2 | `etcdctl endpoint health` |
| HAProxy | Node 1 | `http://192.168.1.10:9999/stats` |

## App Connection Strings

```bash
# MySQL — all traffic to primary (read + write)
mysql -h 192.168.1.10 -P 3306 -u appuser -p'AppPass123!'

# PostgreSQL — all traffic to primary (read + write)
psql -h 192.168.1.10 -p 5432 -U appuser
```