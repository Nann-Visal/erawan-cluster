package mysql

import "time"

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
	SecondaryIPs         []string `json:"secondary_ips"`
	BootstrapRouter      bool     `json:"bootstrap_router"`
	SSHUser              string   `json:"ssh_user"`
	SSHPassword          string   `json:"ssh_password"`
	SSHPort              int      `json:"ssh_port"`
	MySQLPort            int      `json:"mysql_port"`
	RouterBasePort       int      `json:"router_base_port"`
	StepTimeoutSeconds   int      `json:"step_timeout_seconds"`
}

type ResumeRequest struct {
	RootPassword         string `json:"root_password"`
	ClusterAdminPassword string `json:"cluster_admin_password"`
	SSHPassword          string `json:"ssh_password"`
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
	Error             string       `json:"error,omitempty"`
	Request           StoredSpec   `json:"request"`
	Steps             []StepResult `json:"steps"`
}

type StoredSpec struct {
	ClusterAdminUsername string   `json:"cluster_admin_username"`
	ClusterName          string   `json:"cluster_name"`
	PrimaryIP            string   `json:"primary_ip"`
	SecondaryIPs         []string `json:"secondary_ips"`
	BootstrapRouter      bool     `json:"bootstrap_router"`
	SSHUser              string   `json:"ssh_user"`
	SSHPort              int      `json:"ssh_port"`
	MySQLPort            int      `json:"mysql_port"`
	RouterBasePort       int      `json:"router_base_port"`
	StepTimeoutSeconds   int      `json:"step_timeout_seconds"`
}

type SecretInput struct {
	RootPassword         string
	ClusterAdminPassword string
	SSHPassword          string
}
