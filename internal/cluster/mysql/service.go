package mysql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Service struct {
	store  *Store
	runner *Runner
	steps  []step
}

type step struct {
	Name      string
	Tag       string
	Skippable bool
}

func NewService(store *Store, runner *Runner) *Service {
	return &Service{
		store:  store,
		runner: runner,
		steps: []step{
			{Name: "preflight", Tag: "preflight"},
			{Name: "configure_instances", Tag: "configure_instances"},
			{Name: "create_cluster", Tag: "create_cluster"},
			{Name: "add_instances", Tag: "add_instances"},
			{Name: "bootstrap_router", Tag: "bootstrap_router", Skippable: true},
			{Name: "verify_cluster", Tag: "verify_cluster"},
		},
	}
}

func (s *Service) Deploy(ctx context.Context, req DeployRequest) (*Job, error) {
	if err := ValidateDeployRequest(&req); err != nil {
		return nil, err
	}

	job := &Job{
		ID:                newJobID(),
		Status:            JobStatusRunning,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		LastCompletedStep: -1,
		Request: StoredSpec{
			ClusterAdminUsername: req.ClusterAdminUsername,
			ClusterName:          req.ClusterName,
			PrimaryIP:            req.PrimaryIP,
			SecondaryIPs:         req.SecondaryIPs,
			AssumePrepared:       req.AssumePrepared,
			BootstrapRouter:      req.BootstrapRouterEnabled(),
			SSHUser:              req.SSHUser,
			SSHPort:              req.SSHPort,
			MySQLPort:            req.MySQLPort,
			StepTimeoutSeconds:   req.StepTimeoutSeconds,
		},
		Steps: make([]StepResult, 0, len(s.steps)+1),
	}

	if err := s.store.Save(job); err != nil {
		return nil, err
	}

	secrets := SecretInput{
		RootPassword:         req.RootPassword,
		ClusterAdminPassword: req.ClusterAdminPassword,
		SSHPassword:          req.SSHPassword,
	}

	if err := s.executeFrom(ctx, job, 0, secrets); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) Resume(ctx context.Context, jobID string, req ResumeRequest) (*Job, error) {
	secret, err := ValidateResumeSecrets(req)
	if err != nil {
		return nil, err
	}

	job, err := s.store.Load(jobID)
	if err != nil {
		return nil, err
	}
	if job.Status == JobStatusCompleted {
		return nil, fmt.Errorf("job %s already completed", jobID)
	}
	if job.Status == JobStatusRolledBack {
		return nil, fmt.Errorf("job %s already rolled back", jobID)
	}

	startIndex := job.LastCompletedStep + 1
	if startIndex >= len(s.steps) {
		job.Status = JobStatusCompleted
		job.Error = ""
		_ = s.store.Save(job)
		return job, nil
	}

	job.Status = JobStatusRunning
	job.Error = ""
	if err := s.store.Save(job); err != nil {
		return nil, err
	}

	if err := s.executeFrom(ctx, job, startIndex, secret); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) Rollback(ctx context.Context, jobID string, req RollbackRequest) (*Job, error) {
	secret, err := ValidateRollbackSecrets(req)
	if err != nil {
		return nil, err
	}
	job, err := s.store.Load(jobID)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(job.Request.StepTimeoutSeconds) * time.Second
	result := s.runner.RunRollback(ctx, job.ID, job.Request, secret, timeout)
	job.Steps = append(job.Steps, result)

	if result.Status == JobStatusCompleted {
		job.Status = JobStatusRolledBack
		job.Error = ""
	} else {
		job.Status = JobStatusFailed
		job.Error = result.Message
	}
	if err := s.store.Save(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Service) Get(jobID string) (*Job, error) {
	return s.store.Load(jobID)
}

func (s *Service) List(limit int) ([]Job, error) {
	return s.store.List(limit)
}

func (s *Service) executeFrom(ctx context.Context, job *Job, startIndex int, secret SecretInput) error {
	timeout := time.Duration(job.Request.StepTimeoutSeconds) * time.Second
	for i := startIndex; i < len(s.steps); i++ {
		st := s.steps[i]
		if reason, shouldSkip := shouldSkipStep(st, job.Request); shouldSkip {
			job.LastCompletedStep = i
			job.CurrentStep = st.Name
			job.Steps = append(job.Steps, StepResult{
				Name:      st.Name,
				Status:    "skipped",
				StartedAt: time.Now().UTC(),
				EndedAt:   time.Now().UTC(),
				ExitCode:  0,
				Message:   reason,
			})
			if err := s.store.Save(job); err != nil {
				return err
			}
			continue
		}

		job.CurrentStep = st.Name
		if err := s.store.Save(job); err != nil {
			return err
		}

		res := s.runner.RunDeployStep(ctx, runConfig{
			jobID:   job.ID,
			spec:    job.Request,
			secret:  secret,
			step:    st,
			timeout: timeout,
		})
		job.Steps = append(job.Steps, res)

		if res.Status != JobStatusCompleted {
			job.Status = JobStatusFailed
			job.Error = res.Message
			if job.Error == "" {
				job.Error = fmt.Sprintf("step %s failed", st.Name)
			}
			_ = s.store.Save(job)
			return fmt.Errorf(job.Error)
		}

		job.LastCompletedStep = i
		job.Error = ""
		if err := s.store.Save(job); err != nil {
			return err
		}
	}

	job.Status = JobStatusCompleted
	job.CurrentStep = ""
	job.Error = ""
	if err := s.store.Save(job); err != nil {
		return err
	}
	return nil
}

func shouldSkipStep(st step, spec StoredSpec) (string, bool) {
	if st.Skippable && !spec.BootstrapRouter {
		return "bootstrap_router is false", true
	}
	if spec.AssumePrepared && (st.Name == "preflight" || st.Name == "configure_instances") {
		return "assume_prepared is true", true
	}
	return "", false
}

func newJobID() string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
