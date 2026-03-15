<p >
  <img src="doc/assets/A5172f582418f41729f3c587f6a5f95e6w.png" alt="erawan-cluster  logo" width="180"/>
</p>

# erawan-cluster

REST API for automated database cluster lifecycle management and HAProxy configuration.

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22+ |
| HTTP Router | [go-chi/chi](https://github.com/go-chi/chi) |
| Build | Makefile |
| Automation | Ansible |
| Proxy | HAProxy (optional) |
| MySQL Cluster | MySQL InnoDB Cluster + MySQL Shell + MySQL Router |
| PostgreSQL Cluster | PostgreSQL 16 + repmgr + repmgrd |

---

## Features

### MySQL Cluster
- Automated MySQL InnoDB Cluster deployment via Ansible
- Auto-failover using MySQL InnoDB Cluster native HA
- MySQL Router bootstrap and service configuration on DB nodes
- MySQL Shell (`mysqlsh`) for cluster operations (`dba.configure_instance`, `dba.createCluster`, `dba.addInstance`)
- Application database and user provisioning
- Job-based async deployment with resume and rollback support
- Primary-only or multi-node (primary + secondaries) deployment

### PostgreSQL Cluster
- Automated PostgreSQL 16 streaming replication
- Auto-failover using repmgrd daemon monitoring
- Auto-rejoin after node recovery with metadata update
- pg_rewind support for timeline divergence recovery
- Split-brain prevention with node-specific boot delay

### HAProxy (Optional)
- Tenant-based HAProxy config generation and hot reload
- Multi-tenant frontend/backend config per port
- No HAProxy restart required — live reload only

---

## Requirements

### API Host
- Go 1.22+
- `ansible-playbook` installed
- `sshpass` installed (required for SSH password authentication to target nodes)
- SSH access to all target DB nodes
- HAProxy installed (if using proxy features)
- `sudo` permission for HAProxy reload command

### MySQL Target Nodes
- MySQL installed and running
- `mysqlsh` (MySQL Shell) installed
- Nodes reachable from API host via SSH
- MySQL `root` account accessible from API host
- Nodes can reach each other on MySQL port (default 3306)

### PostgreSQL Target Nodes
- PostgreSQL 16 installed
- `repmgr` and `repmgrd` installed (`postgresql-16-repmgr`)
- Nodes reachable from each other on port 5432
- `shared_preload_libraries = 'repmgr'` set in `postgresql.conf`
- `wal_log_hints = on` set in `postgresql.conf`

---

## Quick Start

### Clone
```bash
git clone <your-repo-url> erawan-cluster
cd erawan-cluster
```

### Install Dependencies
```bash
sudo apt update
sudo apt install -y golang-go ansible haproxy sshpass mysql-client
```

### Build and Run
```bash
make tidy
make build
make run
```

Default listen address: `0.0.0.0:8080`

### Health Check
```bash
curl http://127.0.0.1:8080/health
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `API_ADDR` | `:8080` | Listen address |
| `TENANTS_DIR` | `/var/lib/erawan-cluster/haproxy/tenants` | HAProxy tenant config directory |
| `HAPROXY_RELOAD_CMD` | `sudo /bin/systemctl reload haproxy` | HAProxy reload command |
| `CLUSTER_STATE_DIR` | `/var/lib/erawan-cluster/cluster/jobs` | Job state directory |
| `ANSIBLE_PLAYBOOK_BIN` | `ansible-playbook` | Ansible binary path |
| `MYSQL_ANSIBLE_DEBUG` | `false` | Stream live Ansible logs to journal |
| `MYSQL_ANSIBLE_VERBOSITY` | `0` | Ansible verbosity level (1–4) |

---

## Make Commands

```bash
make tidy    # go mod tidy
make fmt     # format source
make test    # run tests
make build   # build binary to ./bin
make run     # run API directly
```

---

## Security

- Request body capped at 1 MiB
- Unknown JSON fields rejected
- IP, port, and username input validation
- Job files stored with restrictive permissions (`0700` dir, `0600` files)
- User input never shell-interpolated