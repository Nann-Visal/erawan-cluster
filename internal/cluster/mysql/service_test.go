package mysql

import "testing"

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
