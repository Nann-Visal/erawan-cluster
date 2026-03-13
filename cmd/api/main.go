package main

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	mysqlcluster "erawan-cluster/internal/cluster/mysql"
	"erawan-cluster/internal/env"
	"erawan-cluster/internal/haproxy"
)

func main() {
	addr := env.GetString("API_ADDR", "")
	if strings.TrimSpace(addr) == "" {
		host := env.GetString("API_HOST", "0.0.0.0")
		port := env.GetString("API_PORT", "8080")
		addr = host + ":" + port
	}

	baseDir := projectBaseDir()
	tenantsDir := env.GetString("TENANTS_DIR", "/var/lib/erawan-cluster/haproxy/tenants")
	reloadCmd := parseCommand(env.GetString("HAPROXY_RELOAD_CMD", "sudo /bin/systemctl reload haproxy"))
	reloadTimeoutSeconds := env.GetInt("HAPROXY_RELOAD_TIMEOUT_SECONDS", 15)

	haproxySvc, err := haproxy.NewService(tenantsDir, reloadCmd, time.Duration(reloadTimeoutSeconds)*time.Second)
	if err != nil {
		log.Fatalf("init haproxy service: %v", err)
	}

	stateDir := env.GetString("CLUSTER_STATE_DIR", "/var/lib/erawan-cluster/cluster/jobs")
	store, err := mysqlcluster.NewStore(stateDir)
	if err != nil {
		log.Fatalf("init mysql cluster store: %v", err)
	}

	ansibleBin := env.GetString("ANSIBLE_PLAYBOOK_BIN", "ansible-playbook")
	deployPlaybook := env.GetString("MYSQL_DEPLOY_PLAYBOOK", filepath.Join(baseDir, "cluster/mysql/playbooks/deploy.yml"))
	rollbackPlaybook := env.GetString("MYSQL_ROLLBACK_PLAYBOOK", filepath.Join(baseDir, "cluster/mysql/playbooks/rollback.yml"))
	mysqlAnsibleDebug := env.GetBool("MYSQL_ANSIBLE_DEBUG", false)
	mysqlAnsibleVerbosity := env.GetInt("MYSQL_ANSIBLE_VERBOSITY", 0)
	mysqlStepOutputMaxChars := env.GetInt("MYSQL_STEP_OUTPUT_MAX_CHARS", 8000)
	if mysqlAnsibleDebug && mysqlAnsibleVerbosity <= 0 {
		mysqlAnsibleVerbosity = 3
	}
	if mysqlAnsibleDebug && mysqlStepOutputMaxChars == 8000 {
		mysqlStepOutputMaxChars = 200000
	}
	runner := mysqlcluster.NewRunner(ansibleBin, deployPlaybook, rollbackPlaybook)
	runner.SetDebug(mysqlAnsibleVerbosity, mysqlAnsibleDebug, mysqlStepOutputMaxChars)
	mysqlSvc := mysqlcluster.NewService(store, runner)

	app := &application{
		config: config{
			addr:   addr,
			env:    env.GetString("ENV", "dev"),
			apiKey: env.GetString("API_KEY", ""),
		},
		haproxy:      haproxySvc,
		mysqlCluster: mysqlSvc,
		baseDir:      baseDir,
	}

	mux := app.mount()
	if mysqlAnsibleDebug {
		log.Printf("mysql ansible debug enabled: verbosity=%d, step_output_max_chars=%d", mysqlAnsibleVerbosity, mysqlStepOutputMaxChars)
	}
	log.Printf("erawan cluster api started at %s", addr)
	log.Fatal(app.run(mux))
}

func parseCommand(raw string) []string {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return []string{"sudo", "/bin/systemctl", "reload", "haproxy"}
	}
	return parts
}
