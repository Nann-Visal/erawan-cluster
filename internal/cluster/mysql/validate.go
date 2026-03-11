package mysql

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
	userPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{1,31}$`)
)

func ValidateDeployRequest(req *DeployRequest) error {
	req.ClusterAdminUsername = strings.TrimSpace(req.ClusterAdminUsername)
	req.ClusterName = strings.TrimSpace(req.ClusterName)
	req.PrimaryIP = strings.TrimSpace(req.PrimaryIP)
	req.SSHUser = strings.TrimSpace(req.SSHUser)

	if strings.TrimSpace(req.RootPassword) == "" {
		return fmt.Errorf("root_password is required")
	}
	if strings.TrimSpace(req.ClusterAdminPassword) == "" {
		return fmt.Errorf("cluster_admin_password is required")
	}
	if strings.TrimSpace(req.SSHPassword) == "" {
		return fmt.Errorf("ssh_password is required")
	}

	if !userPattern.MatchString(req.ClusterAdminUsername) {
		return fmt.Errorf("cluster_admin_username must match %s", userPattern.String())
	}
	if !namePattern.MatchString(req.ClusterName) {
		return fmt.Errorf("cluster_name must match %s", namePattern.String())
	}
	if !userPattern.MatchString(req.SSHUser) {
		return fmt.Errorf("ssh_user must match %s", userPattern.String())
	}

	if net.ParseIP(req.PrimaryIP) == nil {
		return fmt.Errorf("primary_ip must be a valid IP address")
	}
	if len(req.SecondaryIPs) == 0 {
		return fmt.Errorf("secondary_ips must contain at least one IP")
	}
	seen := map[string]struct{}{req.PrimaryIP: {}}
	for i, ip := range req.SecondaryIPs {
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("secondary_ips[%d] must be a valid IP address", i)
		}
		if _, ok := seen[ip]; ok {
			return fmt.Errorf("duplicate IP detected: %s", ip)
		}
		seen[ip] = struct{}{}
		req.SecondaryIPs[i] = ip
	}

	if req.SSHPort <= 0 || req.SSHPort > 65535 {
		return fmt.Errorf("ssh_port must be between 1 and 65535")
	}
	if req.MySQLPort == 0 {
		req.MySQLPort = 3306
	}
	if req.MySQLPort < 1 || req.MySQLPort > 65535 {
		return fmt.Errorf("mysql_port must be between 1 and 65535")
	}
	if req.RouterBasePort == 0 {
		req.RouterBasePort = 6446
	}
	if req.RouterBasePort < 1 || req.RouterBasePort > 65530 {
		return fmt.Errorf("router_base_port must be between 1 and 65530")
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
		RootPassword:         strings.TrimSpace(req.RootPassword),
		ClusterAdminPassword: strings.TrimSpace(req.ClusterAdminPassword),
		SSHPassword:          strings.TrimSpace(req.SSHPassword),
	}
	if secret.RootPassword == "" || secret.ClusterAdminPassword == "" || secret.SSHPassword == "" {
		return SecretInput{}, fmt.Errorf("root_password, cluster_admin_password, and ssh_password are required")
	}
	return secret, nil
}

func ValidateRollbackSecrets(req RollbackRequest) (SecretInput, error) {
	secret := SecretInput{
		RootPassword:         strings.TrimSpace(req.RootPassword),
		ClusterAdminPassword: strings.TrimSpace(req.ClusterAdminPassword),
		SSHPassword:          strings.TrimSpace(req.SSHPassword),
	}
	if secret.RootPassword == "" || secret.ClusterAdminPassword == "" || secret.SSHPassword == "" {
		return SecretInput{}, fmt.Errorf("root_password, cluster_admin_password, and ssh_password are required")
	}
	return secret, nil
}
