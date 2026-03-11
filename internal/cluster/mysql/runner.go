package mysql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	ansibleBin       string
	deployPlaybook   string
	rollbackPlaybook string
}

func NewRunner(ansibleBin, deployPlaybook, rollbackPlaybook string) *Runner {
	if strings.TrimSpace(ansibleBin) == "" {
		ansibleBin = "ansible-playbook"
	}
	return &Runner{
		ansibleBin:       ansibleBin,
		deployPlaybook:   deployPlaybook,
		rollbackPlaybook: rollbackPlaybook,
	}
}

type runConfig struct {
	jobID   string
	spec    StoredSpec
	secret  SecretInput
	step    step
	timeout time.Duration
}

func (r *Runner) RunDeployStep(ctx context.Context, cfg runConfig) StepResult {
	return r.run(ctx, cfg, r.deployPlaybook)
}

func (r *Runner) RunRollback(ctx context.Context, jobID string, spec StoredSpec, secret SecretInput, timeout time.Duration) StepResult {
	cfg := runConfig{
		jobID:  jobID,
		spec:   spec,
		secret: secret,
		step: step{
			Name: "rollback",
			Tag:  "rollback",
		},
		timeout: timeout,
	}
	return r.run(ctx, cfg, r.rollbackPlaybook)
}

func (r *Runner) run(ctx context.Context, cfg runConfig, playbook string) StepResult {
	result := StepResult{
		Name:      cfg.step.Name,
		Status:    JobStatusRunning,
		StartedAt: time.Now().UTC(),
		ExitCode:  -1,
	}
	defer func() { result.EndedAt = time.Now().UTC() }()

	if strings.TrimSpace(playbook) == "" {
		result.Status = JobStatusFailed
		result.Message = "playbook path is not configured"
		return result
	}

	workspace, err := os.MkdirTemp("", "mysql-cluster-job-")
	if err != nil {
		result.Status = JobStatusFailed
		result.Message = fmt.Sprintf("create temp dir: %v", err)
		return result
	}
	defer os.RemoveAll(workspace)

	inventoryPath := filepath.Join(workspace, "inventory.yml")
	varsPath := filepath.Join(workspace, "vars.json")

	if err := os.WriteFile(inventoryPath, []byte(buildInventoryYAML(cfg.spec, cfg.secret)), 0o600); err != nil {
		result.Status = JobStatusFailed
		result.Message = fmt.Sprintf("write inventory: %v", err)
		return result
	}

	extraVars := map[string]any{
		"cluster_name":           cfg.spec.ClusterName,
		"cluster_admin_username": cfg.spec.ClusterAdminUsername,
		"cluster_admin_password": cfg.secret.ClusterAdminPassword,
		"root_password":          cfg.secret.RootPassword,
		"primary_ip":             cfg.spec.PrimaryIP,
		"secondary_ips":          cfg.spec.SecondaryIPs,
		"mysql_port":             cfg.spec.MySQLPort,
		"bootstrap_router":       cfg.spec.BootstrapRouter,
		"router_base_port":       cfg.spec.RouterBasePort,
		"router_service_name":    "mysqlrouter-" + cfg.spec.ClusterName,
	}

	sanitized, err := json.Marshal(extraVars)
	if err != nil {
		result.Status = JobStatusFailed
		result.Message = fmt.Sprintf("marshal vars: %v", err)
		return result
	}

	if err := os.WriteFile(varsPath, sanitized, 0o600); err != nil {
		result.Status = JobStatusFailed
		result.Message = fmt.Sprintf("write vars: %v", err)
		return result
	}

	runTimeout := cfg.timeout
	if runTimeout <= 0 {
		runTimeout = 15 * time.Minute
	}
	stepCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	args := []string{
		"-i", inventoryPath,
		playbook,
		"--tags", cfg.step.Tag,
		"--extra-vars", "@" + varsPath,
	}

	cmd := exec.CommandContext(stepCtx, r.ansibleBin, args...)
	cmd.Env = append(os.Environ(), "ANSIBLE_HOST_KEY_CHECKING=False")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result.Stdout = trimOutput(stdout.String())
	result.Stderr = trimOutput(stderr.String())

	if err == nil {
		result.Status = JobStatusCompleted
		result.ExitCode = 0
		return result
	}

	result.Status = JobStatusFailed
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = 1
	}
	if stepCtx.Err() == context.DeadlineExceeded {
		result.Message = "step execution timed out"
		return result
	}
	result.Message = fmt.Sprintf("ansible step failed: %v", err)
	return result
}

func buildInventoryYAML(spec StoredSpec, secret SecretInput) string {
	var b strings.Builder
	b.WriteString("all:\n")
	b.WriteString("  hosts:\n")
	writeHost := func(name, ip string) {
		b.WriteString("    " + name + ":\n")
		b.WriteString("      ansible_host: " + strconv.Quote(ip) + "\n")
		b.WriteString("      ansible_user: " + strconv.Quote(spec.SSHUser) + "\n")
		b.WriteString("      ansible_password: " + strconv.Quote(secret.SSHPassword) + "\n")
		b.WriteString(fmt.Sprintf("      ansible_port: %d\n", spec.SSHPort))
		b.WriteString("      ansible_become: true\n")
	}

	writeHost("primary", spec.PrimaryIP)
	for i, ip := range spec.SecondaryIPs {
		writeHost(fmt.Sprintf("secondary_%d", i+1), ip)
	}

	b.WriteString("  children:\n")
	b.WriteString("    mysql_primary:\n")
	b.WriteString("      hosts:\n")
	b.WriteString("        primary: {}\n")
	b.WriteString("    mysql_secondary:\n")
	b.WriteString("      hosts:\n")
	for i := range spec.SecondaryIPs {
		b.WriteString(fmt.Sprintf("        secondary_%d: {}\n", i+1))
	}
	return b.String()
}

func trimOutput(in string) string {
	const max = 8000
	in = strings.TrimSpace(in)
	if len(in) <= max {
		return in
	}
	return in[:max] + "\n...truncated..."
}
