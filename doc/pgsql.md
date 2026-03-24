# PostgreSQL HA with Patroni + etcd

This project now deploys PostgreSQL HA with `Patroni` and `etcd` instead of `repmgr/repmgrd`.

## Assumptions

- PostgreSQL is already installed on every node.
- `patroni[etcd]` is already installed.
- `etcd` is already installed.
- Minimum supported topology is 3 PostgreSQL nodes.
- You are deploying a fresh 3-node cluster:
  - `10.0.0.1`
  - `10.0.0.2`
  - `10.0.0.3`

## What the automation writes

On every node the playbooks now create:

- `/etc/etcd/etcd.conf`
- `/etc/systemd/system/etcd.service.d/override.conf`
- `/etc/patroni/patroni.yml`
- `/etc/systemd/system/patroni.service`

The generated Patroni config follows this layout:

- `scope: <cluster_name>`
- `namespace: /db/`
- REST API on `:8008`
- etcd client endpoints on `:2379`
- PostgreSQL on `:5432`

## API payload

Use a PostgreSQL deploy body like this:

```json
{
  "cluster_name": "postgres-cluster",
  "primary_ip": "10.0.0.1",
  "standby_ips": ["10.0.0.2", "10.0.0.3"],
  "postgres_password": "postgrespassword",
  "replicator_password": "replicatorpassword",
  "admin_password": "adminpassword",
  "new_user": "appuser",
  "new_user_password": "AppUser#2026",
  "new_db": "appdb",
  "ssh_user": "root",
  "ssh_password": "password",
  "ssh_port": 22,
  "postgres_port": 5432,
  "step_timeout_seconds": 900
}
```

To resume a failed job:

```json
{
  "postgres_password": "postgrespassword",
  "replicator_password": "replicatorpassword",
  "admin_password": "adminpassword",
  "ssh_password": "password",
  "new_user_password": "AppUser#2026"
}
```

## Deployment flow

1. Preflight checks confirm `psql`, `patroni`, `etcd`, and the PostgreSQL server binaries are present.
2. Base configuration stops the distro-managed PostgreSQL service, installs the `etcd` override, and installs the Patroni systemd unit.
3. Primary and standby steps write node-specific Patroni configs and reset the PostgreSQL data directories for a fresh Patroni bootstrap.
4. Cluster bootstrap starts `etcd` on all nodes, then starts Patroni on the requested primary, then on the standby nodes.
5. Verification checks systemd state, Patroni REST API membership, and `pg_stat_replication`.

## Architecture Overview

```text
                    App Clients / Applications
                               |
                               v
                      +------------------+
                      | Patroni Leader   |
                      | PostgreSQL write |
                      +------------------+
                          |          |
                          | streaming |
                          v          v
                   +-------------+ +-------------+
                   | Patroni     | | Patroni     |
                   | Replica     | | Replica     |
                   +-------------+ +-------------+

          +-------------+   +-------------+   +-------------+
          | etcd node 1 |   | etcd node 2 |   | etcd node 3 |
          +-------------+   +-------------+   +-------------+
                 \                |                /
                  \               |               /
                   +-----------------------------+
                   | DCS / leader election state |
                   +-----------------------------+
```

## Important behavior

- The automation is intended for a fresh cluster bootstrap.
- It clears `/var/lib/etcd` and the PostgreSQL data directory under `/var/lib/postgresql/<major>/main`.
- Patroni becomes the process manager for PostgreSQL; the distro `postgresql@...` service is stopped and disabled.
