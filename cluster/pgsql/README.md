# PostgreSQL Cluster Ansible

This folder contains the PostgreSQL HA cluster automation used by the API.

Supported topologies:

- 1 PostgreSQL primary node only
- 1 PostgreSQL primary node plus 1 or more standby nodes

Implemented workflow:

- PostgreSQL + `patroni` + `etcd` preflight checks
- Shared `etcd` cluster configuration on all PostgreSQL nodes
- Patroni leader bootstrap on the requested primary node
- Patroni replica bootstrap on standby nodes when `standby_ips` is provided
- Retry-safe bootstrap resets data only once per deployment job
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
   | leader + optional|
   | replicas         |
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
