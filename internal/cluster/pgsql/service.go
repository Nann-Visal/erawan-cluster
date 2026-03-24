package pgsql

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
			{Name: "base_config", Tag: "base_config"},
			{Name: "primary_config", Tag: "primary_config"},
			{Name: "standby_config", Tag: "standby_config"},
			{Name: "cluster_bootstrap", Tag: "cluster_bootstrap"},
			{Name: "verify_cluster", Tag: "verify_cluster"},
			{Name: "init_app_db", Tag: "init_app_db", Skippable: true},
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
			ClusterName:        req.ClusterName,
			PrimaryIP:          req.PrimaryIP,
			StandbyIPs:         req.StandbyIPs,
			NewUser:            req.NewUser,
			NewDB:              req.NewDB,
			SSHUser:            req.SSHUser,
			SSHPort:            req.SSHPort,
			PostgresPort:       req.PostgresPort,
			StepTimeoutSeconds: req.StepTimeoutSeconds,
		},
		Steps: make([]StepResult, 0, len(s.steps)),
	}

	if err := s.store.Save(job); err != nil {
		return nil, err
	}

	secrets := SecretInput{
		PostgresPassword:   req.PostgresPassword,
		ReplicatorPassword: req.ReplicatorPassword,
		AdminPassword:      req.AdminPassword,
		SSHPassword:        req.SSHPassword,
		NewUserPassword:    req.NewUserPassword,
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

	startIndex := job.LastCompletedStep + 1
	if startIndex >= len(s.steps) {
		job.Status = JobStatusCompleted
		job.Error = ""
		_ = s.store.Save(job)
		return job, nil
	}
	if job.Request.NewUser != "" && secret.NewUserPassword == "" {
		return nil, fmt.Errorf("new_user_password is required to resume job %s", jobID)
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
	if st.Skippable && (spec.NewUser == "" || spec.NewDB == "") {
		return "new_user/new_db not provided", true
	}
	return "", false
}

func newJobID() string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
