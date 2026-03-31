package mysql

import (
	"context"
	"testing"
	"time"
)

func TestUpdateJobProgressCountsOnlyApplicableCompletedSteps(t *testing.T) {
	svc := NewService(nil, nil)
	job := &Job{
		Status: JobStatusRunning,
		Request: StoredSpec{
			AssumePrepared:  true,
			BootstrapRouter: false,
		},
		Steps: []StepResult{
			{Name: "preflight", Status: "skipped"},
			{Name: "configure_instances", Status: "skipped"},
			{Name: "create_cluster", Status: JobStatusCompleted},
			{Name: "add_instances", Status: "skipped"},
		},
	}

	svc.updateJobProgress(job)

	if job.TotalSteps != 2 {
		t.Fatalf("expected total_steps=2, got %d", job.TotalSteps)
	}
	if job.CompletedSteps != 1 {
		t.Fatalf("expected completed_steps=1, got %d", job.CompletedSteps)
	}
	if job.ProgressPercent != 50 {
		t.Fatalf("expected progress_percent=50, got %d", job.ProgressPercent)
	}
}

func TestUpdateJobProgressCompletedJobsReportOneHundredPercent(t *testing.T) {
	svc := NewService(nil, nil)
	job := &Job{
		Status: JobStatusCompleted,
		Request: StoredSpec{
			BootstrapRouter: true,
		},
	}

	svc.updateJobProgress(job)

	if job.TotalSteps != 5 {
		t.Fatalf("expected total_steps=5, got %d", job.TotalSteps)
	}
	if job.CompletedSteps != 5 {
		t.Fatalf("expected completed_steps=5, got %d", job.CompletedSteps)
	}
	if job.ProgressPercent != 100 {
		t.Fatalf("expected progress_percent=100, got %d", job.ProgressPercent)
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
		RootPassword:         "rootpassword",
		ClusterAdminUsername: "clusteradmin",
		ClusterAdminPassword: "clusterpassword",
		ClusterName:          "prodCluster",
		PrimaryIP:            "10.0.0.1",
		SSHUser:              "root",
		SSHPassword:          "password",
		SSHPort:              22,
		StepTimeoutSeconds:   30,
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
			ClusterAdminUsername: "clusteradmin",
			ClusterName:          "prodCluster",
			PrimaryIP:            "10.0.0.1",
			SSHUser:              "root",
			SSHPort:              22,
			MySQLPort:            3306,
			StepTimeoutSeconds:   30,
		},
	}
	if err := store.Save(job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	resumed, err := svc.Resume(context.Background(), job.ID, ResumeRequest{
		RootPassword:         "rootpassword",
		ClusterAdminPassword: "clusterpassword",
		SSHPassword:          "password",
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

func TestShouldSkipStepSkipsAddInstancesWhenNoStandbys(t *testing.T) {
	reason, skip := shouldSkipStep(step{Name: "add_instances"}, StoredSpec{})
	if !skip {
		t.Fatal("expected add_instances to be skipped when standby_ips is empty")
	}
	if reason != "standby_ips is empty" {
		t.Fatalf("unexpected skip reason: %q", reason)
	}
}
