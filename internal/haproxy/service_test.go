package haproxy

import (
	"strings"
	"testing"
)

func TestBuildConfigContentUsesFailoverFriendlyTCPSettings(t *testing.T) {
	cfg := buildConfigContent(25010, []string{"10.10.255.102", "10.10.146.139"}, 6446)

	required := []string{
		"listen node_25010",
		"# Always use first available healthy server = deterministic primary routing",
		"balance first",
		"# TCP keepalive",
		"option clitcpka",
		"option srvtcpka",
		"timeout connect  5s",
		"timeout check    2s",
		"timeout queue    10s",
		"timeout client   10m",
		"timeout server   10m",
		"option redispatch",
		"retries 3",
		"default-server inter 2s fastinter 500ms downinter 1s fall 3 rise 2 on-marked-down shutdown-sessions on-marked-up shutdown-backup-sessions",
		"# MySQL Router write port — port 6446 = R/W, always primary",
		"# Use first server as primary, others as backup",
		"server db1 10.10.255.102:6446 check",
		"server db2 10.10.146.139:6446 check backup",
	}

	for _, want := range required {
		if !strings.Contains(cfg, want) {
			t.Fatalf("expected config to contain %q\nfull config:\n%s", want, cfg)
		}
	}
}
