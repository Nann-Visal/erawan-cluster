# MySQL InnoDB Cluster with MySQL Router

This project deploys MySQL HA with MySQL InnoDB Cluster, MySQL Shell, and optional MySQL Router bootstrap.

## Assumptions

- MySQL is already installed and running on every target node.
- `mysqlsh` is already installed on every target node.
- MySQL nodes can reach each other on the MySQL port.
- The API host can SSH to every target node.

Supported topologies:

- Single-node bootstrap:
  - 1 primary node
- HA cluster:
  - 1 primary node
  - 1 or more secondary nodes

## API payload

Use a MySQL deploy body like this:

```json
{
  "root_password": "RootPass#2026",
  "cluster_admin_username": "clusteradmin",
  "cluster_admin_password": "ClusterAdmin#2026",
  "cluster_name": "prodCluster",
  "primary_ip": "192.168.122.154",
  "secondary_ips": ["192.168.122.111"],
  "new_user": "appuser",
  "new_user_password": "AppUser#2026",
  "new_user_ssl_required": true,
  "new_db": "appdb",
  "assume_prepared": false,
  "bootstrap_router": true,
  "ssh_user": "root",
  "ssh_password": "password",
  "ssh_port": 22,
  "mysql_port": 3306,
  "step_timeout_seconds": 900
}
```

To resume a failed job:

```json
{
  "root_password": "RootPass#2026",
  "cluster_admin_password": "ClusterAdmin#2026",
  "ssh_password": "password",
  "new_user_password": "AppUser#2026"
}
```

To roll back a MySQL job:

```json
{
  "root_password": "RootPass#2026",
  "cluster_admin_password": "ClusterAdmin#2026",
  "ssh_password": "password"
}
```

## Field behavior

- `primary_ip`: node used to create the initial InnoDB Cluster.
- `secondary_ips`: optional list of replica nodes to add after cluster creation.
- `bootstrap_router`: when `true`, bootstraps MySQL Router on all DB nodes.
- `assume_prepared`: when `true`, skips preflight and instance-configuration steps.
- `new_user`, `new_user_password`, `new_db`: optional application database bootstrap.
- `new_user_ssl_required`: controls whether the created MySQL user requires SSL.

## Deployment flow

1. Preflight checks confirm MySQL, MySQL Shell, and connectivity prerequisites are present.
2. Instance configuration prepares each node for InnoDB Cluster and creates or updates the cluster admin account.
3. Cluster creation runs on the requested primary node.
4. Secondary nodes are added with clone-based recovery when `secondary_ips` is not empty.
5. MySQL Router is bootstrapped on all nodes when `bootstrap_router` is enabled.
6. Verification checks cluster health and router state.
7. Optional application database and user creation runs on the primary.

## Architecture Overview

```text
                         App Clients
                              |
                              v
                        +------------+
                        |  HAProxy    |
                        |  optional   |
                        +------------+
                              |
                              v
                  +-------------------------+
                  | MySQL Router on DB nodes|
                  | optional bootstrap      |
                  +-------------------------+
                     |            |            |
                     v            v            v
                +---------+  +---------+  +---------+
                | Primary |  |Secondary|  |Secondary|
                | MySQL   |  | MySQL   |  | MySQL   |
                +---------+  +---------+  +---------+
                     \            |            /
                      \           |           /
                       +---------------------+
                       | InnoDB Cluster GR   |
                       | managed by mysqlsh  |
                       +---------------------+
```

## Optional modes

Single-node mode:

- Leave `secondary_ips` empty.
- The `add_instances` step is skipped automatically.

Prepared-node mode:

- Set `assume_prepared` to `true` if the nodes were already prepared earlier.
- The `preflight` and `configure_instances` steps are skipped.

No-router mode:

- Set `bootstrap_router` to `false`.
- The router bootstrap step is skipped.

## What the automation manages

The MySQL playbooks manage:

- InnoDB Cluster lifecycle through `mysqlsh`
- Cluster member addition on secondary nodes
- MySQL Router bootstrap and systemd service installation
- Optional application database and user creation
- Rollback for router services and cluster dissolve

## Important behavior

- MySQL supports both single-node and multi-node deployments in this project.
- Rollback support exists for MySQL jobs through the rollback API.
- If `bootstrap_router` is enabled, router services are created on all target DB nodes.
