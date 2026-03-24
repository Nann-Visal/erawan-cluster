# MySQL Cluster Ansible

This folder contains the MySQL InnoDB Cluster automation used by the API.

Supported topologies:

- 1 MySQL primary node only
- 1 MySQL primary node plus 1 or more secondary nodes

Implemented workflow:

- MySQL and MySQL Shell preflight checks
- Instance preparation for InnoDB Cluster
- Cluster creation on the requested primary node
- Secondary-node addition when `secondary_ips` is provided
- Optional MySQL Router bootstrap on all DB nodes
- Cluster verification and optional application database bootstrap
- Rollback playbook for router cleanup and cluster dissolve

Architecture overview:

```text
      API / Ansible
           |
           v
   +----------------+
   | mysqlsh        |
   | cluster tasks  |
   +----------------+
           |
           v
   +---------------------+
   | InnoDB Cluster      |
   | primary + secondary |
   +---------------------+
           |
           v
   +---------------------+
   | MySQL Router        |
   | optional per node   |
   +---------------------+
```

Entry points:

- `cluster/mysql/playbooks/deploy.yml`
- `cluster/mysql/playbooks/rollback.yml`
