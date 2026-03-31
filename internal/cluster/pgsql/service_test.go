package pgsql

import (
	"context"
	"testing"
	"time"
)

func TestValidateDeployRequestAllowsPrimaryOnlyTopology(t *testing.T) {
	req := DeployRequest{
		PrimaryIP:          "10.0.0.1",
		StandbyIPs:         []string{},
		PostgresPassword:   "postgrespassword",
		ReplicatorPassword: "replicatorpassword",
		AdminPassword:      "adminpassword",
		SSHUser:            "root",
		SSHPassword:        "password",
	}

	if err := ValidateDeployRequest(&req); err != nil {
		t.Fatalf("expected primary-only topology to validate, got error: %v", err)
	}
	if req.SSHPort != 22 {
		t.Fatalf("expected default ssh_port=22, got %d", req.SSHPort)
	}
	if req.PostgresPort != 5432 {
		t.Fatalf("expected default postgres_port=5432, got %d", req.PostgresPort)
	}
	if !req.NewUserSSLRequiredEnabled() {
		t.Fatal("expected new_user_ssl_required to default to true")
	}
}

func TestShouldSkipStepSkipsStandbyConfigWhenNoStandbys(t *testing.T) {
	reason, skip := shouldSkipStep(step{Name: "standby_config"}, StoredSpec{})
	if !skip {
		t.Fatal("expected standby_config to be skipped when standby_ips is empty")
	}
	if reason != "standby_ips is empty" {
		t.Fatalf("unexpected skip reason: %q", reason)
	}
}

func TestDeploySchedulesBackgroundExecution(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	svc := NewService(store, nil)
	launched := false
	svc.start = func(fn func()) {
		launched = true
	}

	job, err := svc.Deploy(context.Background(), DeployRequest{
		ClusterName:        "postgres-cluster",
		PrimaryIP:          "10.0.0.1",
		PostgresPassword:   "postgrespassword",
		ReplicatorPassword: "replicatorpassword",
		AdminPassword:      "adminpassword",
		SSHUser:            "root",
		SSHPassword:        "password",
		SSHPort:            22,
		StepTimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if !launched {
		t.Fatal("expected deploy to launch background execution")
	}
	if job.Status != JobStatusRunning {
		t.Fatalf("expected running job status, got %q", job.Status)
	}
	if len(job.Steps) != 0 {
		t.Fatalf("expected no steps to run inline, got %d", len(job.Steps))
	}

	saved, err := store.Load(job.ID)
	if err != nil {
		t.Fatalf("load saved job: %v", err)
	}
	if saved.ID != job.ID {
		t.Fatalf("expected saved job id %q, got %q", job.ID, saved.ID)
	}
	if !saved.Request.NewUserSSLRequired {
		t.Fatal("expected stored request to default new_user_ssl_required to true")
	}
}

func TestResumeSchedulesBackgroundExecution(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	svc := NewService(store, nil)
	launched := false
	svc.start = func(fn func()) {
		launched = true
	}

	job := &Job{
		ID:                "job1",
		Status:            JobStatusFailed,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		LastCompletedStep: 0,
		Request: StoredSpec{
			ClusterName:        "postgres-cluster",
			PrimaryIP:          "10.0.0.1",
			SSHUser:            "root",
			SSHPort:            22,
			PostgresPort:       5432,
			StepTimeoutSeconds: 30,
		},
	}
	if err := store.Save(job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	resumed, err := svc.Resume(context.Background(), job.ID, ResumeRequest{
		PostgresPassword:   "postgrespassword",
		ReplicatorPassword: "replicatorpassword",
		AdminPassword:      "adminpassword",
		SSHPassword:        "password",
	})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !launched {
		t.Fatal("expected resume to launch background execution")
	}
	if resumed.Status != JobStatusRunning {
		t.Fatalf("expected running job status, got %q", resumed.Status)
	}
}
