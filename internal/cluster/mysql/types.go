package mysql

import (
	"encoding/json"
	"time"
)

const (
	JobStatusPending    = "pending"
	JobStatusRunning    = "running"
	JobStatusFailed     = "failed"
	JobStatusCompleted  = "completed"
	JobStatusRolledBack = "rolled_back"
)

type DeployRequest struct {
	RootPassword         string   `json:"root_password"`
	ClusterAdminUsername string   `json:"cluster_admin_username"`
	ClusterAdminPassword string   `json:"cluster_admin_password"`
	ClusterName          string   `json:"cluster_name"`
	PrimaryIP            string   `json:"primary_ip"`
	StandbyIPs           []string `json:"standby_ips"`
	SecondaryIPs         []string `json:"secondary_ips,omitempty"`
	NewUser              string   `json:"new_user"`
	NewUserPassword      string   `json:"new_user_password"`
	NewUserSSLRequired   bool     `json:"new_user_ssl_required"`
	NewDB                string   `json:"new_db"`
	AssumePrepared       bool     `json:"assume_prepared"`
	BootstrapRouter      *bool    `json:"bootstrap_router"`
	SSHUser              string   `json:"ssh_user"`
	SSHPassword          string   `json:"ssh_password"`
	SSHPort              int      `json:"ssh_port"`
	MySQLPort            int      `json:"mysql_port"`
	StepTimeoutSeconds   int      `json:"step_timeout_seconds"`
}

func (r DeployRequest) BootstrapRouterEnabled() bool {
	if r.BootstrapRouter == nil {
		return true
	}
	return *r.BootstrapRouter
}

type ResumeRequest struct {
	RootPassword         string `json:"root_password"`
	ClusterAdminPassword string `json:"cluster_admin_password"`
	SSHPassword          string `json:"ssh_password"`
	NewUserPassword      string `json:"new_user_password"`
}

type RollbackRequest struct {
	RootPassword         string `json:"root_password"`
	ClusterAdminPassword string `json:"cluster_admin_password"`
	SSHPassword          string `json:"ssh_password"`
}

type StepResult struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	ExitCode  int       `json:"exit_code"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type Job struct {
	ID                string       `json:"id"`
	Status            string       `json:"status"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	CurrentStep       string       `json:"current_step,omitempty"`
	LastCompletedStep int          `json:"last_completed_step"`
	CompletedSteps    int          `json:"completed_steps"`
	TotalSteps        int          `json:"total_steps"`
	ProgressPercent   int          `json:"progress_percent"`
	Error             string       `json:"error,omitempty"`
	Request           StoredSpec   `json:"request"`
	Steps             []StepResult `json:"steps"`
}

type StoredSpec struct {
	ClusterAdminUsername string   `json:"cluster_admin_username"`
	ClusterName          string   `json:"cluster_name"`
	PrimaryIP            string   `json:"primary_ip"`
	StandbyIPs           []string `json:"standby_ips"`
	NewUser              string   `json:"new_user"`
	NewUserSSLRequired   bool     `json:"new_user_ssl_required"`
	NewDB                string   `json:"new_db"`
	AssumePrepared       bool     `json:"assume_prepared"`
	BootstrapRouter      bool     `json:"bootstrap_router"`
	SSHUser              string   `json:"ssh_user"`
	SSHPort              int      `json:"ssh_port"`
	MySQLPort            int      `json:"mysql_port"`
	StepTimeoutSeconds   int      `json:"step_timeout_seconds"`
}

type SecretInput struct {
	RootPassword         string
	ClusterAdminPassword string
	SSHPassword          string
	NewUserPassword      string
}

type StoredSecret struct {
	ClusterAdminPassword string `json:"cluster_admin_password"`
}

func (s *StoredSpec) UnmarshalJSON(data []byte) error {
	type alias StoredSpec
	aux := struct {
		*alias
		SecondaryIPs []string `json:"secondary_ips"`
	}{
		alias: (*alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(s.StandbyIPs) == 0 && len(aux.SecondaryIPs) > 0 {
		s.StandbyIPs = aux.SecondaryIPs
	}
	return nil
}
