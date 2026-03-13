# Ubuntu Production Install (22.04/24.04)

## 1) Prepare host
```bash
sudo apt update
sudo apt install -y git curl
```

## 2) Get source and build binary
```bash
git clone <repo-url> erawan-cluster
cd erawan-cluster
make build
```

## 3) Run installer
```bash
sudo bash scripts/install-ubuntu.sh
```

If your binary or cluster folder is in custom path:
```bash
sudo BIN_SRC=/path/to/erawan-cluster CLUSTER_SRC=/path/to/cluster bash scripts/install-ubuntu.sh
```

## 4) Configure API env
```bash
sudo nano /etc/erawan-cluster/.env
```

Set at minimum:
```env
ENV=prod
API_HOST=127.0.0.1
API_PORT=8080
API_KEY=<long-random-key>
TENANTS_DIR=/var/lib/erawan-cluster/haproxy/tenants
HAPROXY_RELOAD_CMD=sudo /bin/systemctl reload haproxy
CLUSTER_STATE_DIR=/var/lib/erawan-cluster/cluster/jobs
MYSQL_DEPLOY_PLAYBOOK=/opt/erawan-cluster/cluster/mysql/playbooks/deploy.yml
MYSQL_ROLLBACK_PLAYBOOK=/opt/erawan-cluster/cluster/mysql/playbooks/rollback.yml
```

## 5) Reload services
```bash
sudo systemctl daemon-reload
sudo systemctl reload haproxy || sudo systemctl start haproxy
sudo systemctl restart erawan-cluster
```

## 6) Verify
```bash
sudo systemctl status erawan-cluster --no-pager
sudo systemctl status haproxy --no-pager
curl -s http://127.0.0.1:8080/health
sudo ss -lntp | grep -E ':8080|:25000|:6446' || true
```

## 7) Live logs
```bash
sudo journalctl -u erawan-cluster -f
sudo journalctl -u haproxy -f
```

## Ubuntu HAProxy notes
1. Ensure `/etc/haproxy/haproxy.cfg` has:
   - `stats socket /run/haproxy/admin.sock mode 660 level admin expose-fd listeners`
2. Installer writes systemd override:
   - `/etc/systemd/system/haproxy.service.d/override.conf`
3. Verify running HAProxy includes both config paths:
   - `/etc/haproxy/haproxy.cfg`
   - `/var/lib/erawan-cluster/haproxy/tenants`
