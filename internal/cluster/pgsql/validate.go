package pgsql

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
	userPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{1,31}$`)
	dbPattern   = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
)

func ValidateDeployRequest(req *DeployRequest) error {
	req.ClusterName = strings.TrimSpace(req.ClusterName)
	req.PrimaryIP = strings.TrimSpace(req.PrimaryIP)
	req.SSHUser = strings.TrimSpace(req.SSHUser)
	req.NewUser = strings.TrimSpace(req.NewUser)
	req.NewDB = strings.TrimSpace(req.NewDB)
	if req.ClusterName == "" {
		req.ClusterName = "postgres-cluster"
	}

	if strings.TrimSpace(req.PostgresPassword) == "" {
		return fmt.Errorf("postgres_password is required")
	}
	if strings.TrimSpace(req.ReplicatorPassword) == "" {
		return fmt.Errorf("replicator_password is required")
	}
	if strings.TrimSpace(req.AdminPassword) == "" {
		return fmt.Errorf("admin_password is required")
	}
	if strings.TrimSpace(req.SSHPassword) == "" {
		return fmt.Errorf("ssh_password is required")
	}
	if !namePattern.MatchString(req.ClusterName) {
		return fmt.Errorf("cluster_name must match %s", namePattern.String())
	}
	if !userPattern.MatchString(req.SSHUser) {
		return fmt.Errorf("ssh_user must match %s", userPattern.String())
	}
	if req.NewUser != "" && !userPattern.MatchString(req.NewUser) {
		return fmt.Errorf("new_user must match %s", userPattern.String())
	}
	if req.NewDB != "" && !dbPattern.MatchString(req.NewDB) {
		return fmt.Errorf("new_db must match %s", dbPattern.String())
	}

	hasInitDBInput := req.NewUser != "" || req.NewDB != "" || strings.TrimSpace(req.NewUserPassword) != ""
	if hasInitDBInput {
		if req.NewUser == "" {
			return fmt.Errorf("new_user is required when init DB fields are provided")
		}
		if strings.TrimSpace(req.NewUserPassword) == "" {
			return fmt.Errorf("new_user_password is required when init DB fields are provided")
		}
		if req.NewDB == "" {
			return fmt.Errorf("new_db is required when init DB fields are provided")
		}
	}

	if net.ParseIP(req.PrimaryIP) == nil {
		return fmt.Errorf("primary_ip must be a valid IP address")
	}
	if len(req.StandbyIPs) < 2 {
		return fmt.Errorf("PostgreSQL Patroni/etcd cluster requires at least 3 nodes total: 1 primary and at least 2 standby nodes")
	}

	seen := map[string]struct{}{req.PrimaryIP: {}}
	for i, ip := range req.StandbyIPs {
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("standby_ips[%d] must be a valid IP address", i)
		}
		if _, ok := seen[ip]; ok {
			return fmt.Errorf("duplicate IP detected: %s", ip)
		}
		seen[ip] = struct{}{}
		req.StandbyIPs[i] = ip
	}

	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.SSHPort < 1 || req.SSHPort > 65535 {
		return fmt.Errorf("ssh_port must be between 1 and 65535")
	}
	if req.PostgresPort == 0 {
		req.PostgresPort = 5432
	}
	if req.PostgresPort < 1 || req.PostgresPort > 65535 {
		return fmt.Errorf("postgres_port must be between 1 and 65535")
	}
	if req.StepTimeoutSeconds == 0 {
		req.StepTimeoutSeconds = 900
	}
	if req.StepTimeoutSeconds < 30 || req.StepTimeoutSeconds > 7200 {
		return fmt.Errorf("step_timeout_seconds must be between 30 and 7200")
	}

	return nil
}

func ValidateResumeSecrets(req ResumeRequest) (SecretInput, error) {
	secret := SecretInput{
		PostgresPassword:   strings.TrimSpace(req.PostgresPassword),
		ReplicatorPassword: strings.TrimSpace(req.ReplicatorPassword),
		AdminPassword:      strings.TrimSpace(req.AdminPassword),
		SSHPassword:        strings.TrimSpace(req.SSHPassword),
		NewUserPassword:    strings.TrimSpace(req.NewUserPassword),
	}
	if secret.PostgresPassword == "" || secret.ReplicatorPassword == "" || secret.AdminPassword == "" || secret.SSHPassword == "" {
		return SecretInput{}, fmt.Errorf("postgres_password, replicator_password, admin_password, and ssh_password are required")
	}
	return secret, nil
}
