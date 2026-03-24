# PostgreSQL Cluster Ansible

This folder contains the PostgreSQL HA cluster automation used by the API.

Minimum supported topology:

- 3 PostgreSQL nodes
- 1 requested primary node
- At least 2 standby nodes

Implemented workflow:

- PostgreSQL + `patroni` + `etcd` preflight checks
- Shared `etcd` cluster configuration on all PostgreSQL nodes
- Patroni leader bootstrap on the requested primary node
- Patroni replica bootstrap on standby nodes
- Cluster verification through systemd state, Patroni REST API, and replication checks
- Optional application database/user bootstrap

Architecture overview:

```text
      API / Ansible
           |
           v
   +------------------+
   | Patroni services |
   | on all PG nodes  |
   +------------------+
           |
           v
   +------------------+
   | PostgreSQL       |
   | leader + replicas|
   +------------------+
           ^
           |
   +------------------+
   | etcd cluster     |
   | shared DCS state |
   +------------------+
```

Entry point:

- `cluster/pgsql/playbooks/deploy.yml`
