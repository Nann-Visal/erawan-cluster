package pgsql

import "testing"

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
