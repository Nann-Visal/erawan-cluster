package haproxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	minPort = 1
	maxPort = 65535
)

type Service struct {
	tenantsDir    string
	reloadCmd     []string
	reloadTimeout time.Duration
}

func NewService(tenantsDir string, reloadCmd []string, reloadTimeout time.Duration) (*Service, error) {
	if strings.TrimSpace(tenantsDir) == "" {
		return nil, fmt.Errorf("tenants directory is required")
	}
	if len(reloadCmd) == 0 {
		return nil, fmt.Errorf("reload command is required")
	}
	if reloadTimeout <= 0 {
		reloadTimeout = 15 * time.Second
	}
	if err := os.MkdirAll(tenantsDir, 0o775); err != nil {
		return nil, fmt.Errorf("create tenants directory: %w", err)
	}
	return &Service{tenantsDir: tenantsDir, reloadCmd: reloadCmd, reloadTimeout: reloadTimeout}, nil
}

type CreateConfigInput struct {
	Port    int      `json:"port"`
	NodeIPs []string `json:"node_ips"`
	DBPort  int      `json:"db_port"`
}

type DeleteConfigInput struct {
	Port int `json:"port"`
}

func ValidatePort(port int, field string) error {
	if port < minPort || port > maxPort {
		return fmt.Errorf("%s must be between %d and %d", field, minPort, maxPort)
	}
	return nil
}

func NormalizeNodeIPs(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("node_ips must contain at least one IP address")
	}

	normalized := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for i, host := range raw {
		host = strings.TrimSpace(host)
		if host == "" {
			return nil, fmt.Errorf("node_ips[%d] cannot be empty", i+1)
		}
		if strings.ContainsAny(host, " \n\r\t") {
			return nil, fmt.Errorf("node_ips[%d] contains invalid whitespace", i+1)
		}
		if ip := net.ParseIP(host); ip == nil {
			return nil, fmt.Errorf("node_ips[%d] must be a valid IP address", i+1)
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("node_ips must contain at least one IP address")
	}
	return normalized, nil
}

func (s *Service) CreateConfig(ctx context.Context, in CreateConfigInput) error {
	if err := ValidatePort(in.Port, "port"); err != nil {
		return err
	}
	if err := ValidatePort(in.DBPort, "db_port"); err != nil {
		return err
	}
	nodes, err := NormalizeNodeIPs(in.NodeIPs)
	if err != nil {
		return err
	}

	filename := s.filename(in.Port)
	content := buildConfigContent(in.Port, nodes, in.DBPort)
	backup := ""

	if _, err := os.Stat(filename); err == nil {
		backup = filename + ".bak"
		if err := copyFile(filename, backup); err != nil {
			return fmt.Errorf("backup existing config: %w", err)
		}
	}

	if err := os.WriteFile(filename, []byte(content), 0o664); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if err := s.Reload(ctx); err != nil {
		if backup != "" {
			_ = os.Rename(backup, filename)
		} else {
			_ = os.Remove(filename)
		}
		return fmt.Errorf("haproxy reload failed, rolled back: %w", err)
	}

	if backup != "" {
		_ = os.Remove(backup)
	}
	return nil
}

func (s *Service) DeleteConfig(ctx context.Context, in DeleteConfigInput) (bool, error) {
	if err := ValidatePort(in.Port, "port"); err != nil {
		return false, err
	}

	filename := s.filename(in.Port)
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat config: %w", err)
	}

	backup := filename + ".bak"
	if err := copyFile(filename, backup); err != nil {
		return false, fmt.Errorf("backup config before delete: %w", err)
	}
	if err := os.Remove(filename); err != nil {
		return false, fmt.Errorf("remove config: %w", err)
	}

	if err := s.Reload(ctx); err != nil {
		_ = os.Rename(backup, filename)
		return false, fmt.Errorf("haproxy reload failed, rolled back deletion: %w", err)
	}
	_ = os.Remove(backup)

	return true, nil
}

func (s *Service) ListConfigs() ([]string, error) {
	entries, err := os.ReadDir(s.tenantsDir)
	if err != nil {
		return nil, fmt.Errorf("read tenants directory: %w", err)
	}
	files := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".cfg") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func (s *Service) Reload(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.reloadTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, s.reloadCmd[0], s.reloadCmd[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("reload failed: %s", msg)
	}
	return nil
}

func (s *Service) filename(port int) string {
	return filepath.Join(s.tenantsDir, fmt.Sprintf("%d.cfg", port))
}

func buildConfigContent(port int, nodeIPs []string, dbPort int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("listen node_%d\n", port))
	b.WriteString(fmt.Sprintf("    bind *:%d\n", port))
	b.WriteString("    mode tcp\n")
	b.WriteString("    balance leastconn\n")
	b.WriteString("    option tcp-check\n")
	b.WriteString("    timeout connect 3s\n")
	b.WriteString("    timeout client  30s\n")
	b.WriteString("    timeout server  30s\n")
	b.WriteString("    option redispatch\n")
	b.WriteString("    retries 3\n")
	for i, ip := range nodeIPs {
		b.WriteString(fmt.Sprintf("    server db%d %s:%d check inter 2s fall 3 rise 2\n", i+1, ip, dbPort))
	}
	return b.String()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o664)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Sync()
}
