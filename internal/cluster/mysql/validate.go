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
	dbPattern   = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
)

func ValidateDeployRequest(req *DeployRequest) error {
	req.ClusterAdminUsername = strings.TrimSpace(req.ClusterAdminUsername)
	req.ClusterName = strings.TrimSpace(req.ClusterName)
	req.PrimaryIP = strings.TrimSpace(req.PrimaryIP)
	req.SSHUser = strings.TrimSpace(req.SSHUser)
	req.NewUser = strings.TrimSpace(req.NewUser)
	req.NewDB = strings.TrimSpace(req.NewDB)
	if req.ClusterName == "" {
		req.ClusterName = "prodCluster"
	}
	if req.ClusterAdminUsername == "" {
		req.ClusterAdminUsername = "clusteradmin"
	}
	if len(req.StandbyIPs) == 0 && len(req.SecondaryIPs) > 0 {
		req.StandbyIPs = req.SecondaryIPs
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

	if req.SSHPort <= 0 || req.SSHPort > 65535 {
		return fmt.Errorf("ssh_port must be between 1 and 65535")
	}
	if req.MySQLPort == 0 {
		req.MySQLPort = 3306
	}
	if req.MySQLPort < 1 || req.MySQLPort > 65535 {
		return fmt.Errorf("mysql_port must be between 1 and 65535")
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
		NewUserPassword:      strings.TrimSpace(req.NewUserPassword),
	}
	if secret.SSHPassword == "" {
		return SecretInput{}, fmt.Errorf("ssh_password is required")
	}
	return secret, nil
}

func ValidateRollbackSecrets(req RollbackRequest) (SecretInput, error) {
	secret := SecretInput{
		RootPassword:         strings.TrimSpace(req.RootPassword),
		ClusterAdminPassword: strings.TrimSpace(req.ClusterAdminPassword),
		SSHPassword:          strings.TrimSpace(req.SSHPassword),
	}
	if secret.SSHPassword == "" {
		return SecretInput{}, fmt.Errorf("ssh_password is required")
	}
	return secret, nil
}
