# PostgreSQL Cluster Ansible

This folder contains the PostgreSQL HA cluster automation used by the API.

Implemented workflow:

- PostgreSQL 16 + `repmgr` preflight checks
- Primary + standby configuration
- Automatic failover via `repmgrd`
- Automatic rejoin after reboot/recovery via `pg-auto-rejoin.service`
- Optional application database/user bootstrap

Entry point:

- `cluster/pgsql/playbooks/deploy.yml`
