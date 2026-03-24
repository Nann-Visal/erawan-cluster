package pgsql

import "time"

const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusFailed    = "failed"
	JobStatusCompleted = "completed"
)

type DeployRequest struct {
	ClusterName        string   `json:"cluster_name"`
	PrimaryIP          string   `json:"primary_ip"`
	StandbyIPs         []string `json:"standby_ips"`
	PostgresPassword   string   `json:"postgres_password"`
	ReplicatorPassword string   `json:"replicator_password"`
	AdminPassword      string   `json:"admin_password"`
	NewUser            string   `json:"new_user"`
	NewUserPassword    string   `json:"new_user_password"`
	NewDB              string   `json:"new_db"`
	SSHUser            string   `json:"ssh_user"`
	SSHPassword        string   `json:"ssh_password"`
	SSHPort            int      `json:"ssh_port"`
	PostgresPort       int      `json:"postgres_port"`
	StepTimeoutSeconds int      `json:"step_timeout_seconds"`
}

type ResumeRequest struct {
	PostgresPassword   string `json:"postgres_password"`
	ReplicatorPassword string `json:"replicator_password"`
	AdminPassword      string `json:"admin_password"`
	SSHPassword        string `json:"ssh_password"`
	NewUserPassword    string `json:"new_user_password"`
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
	ClusterName        string   `json:"cluster_name"`
	PrimaryIP          string   `json:"primary_ip"`
	StandbyIPs         []string `json:"standby_ips"`
	NewUser            string   `json:"new_user"`
	NewDB              string   `json:"new_db"`
	SSHUser            string   `json:"ssh_user"`
	SSHPort            int      `json:"ssh_port"`
	PostgresPort       int      `json:"postgres_port"`
	StepTimeoutSeconds int      `json:"step_timeout_seconds"`
}

type SecretInput struct {
	PostgresPassword   string
	ReplicatorPassword string
	AdminPassword      string
	SSHPassword        string
	NewUserPassword    string
}
