package platform

import (
	"strings"
	"testing"
)

func TestSandboxProfiles(t *testing.T) {
	args := []string{"run", "--pids-limit", "128", "--cpus", "1", "--read-only", "--cap-drop", "ALL", "image"}
	t.Setenv("TOOLDECK_SANDBOX_PROFILE", "")
	strict := strings.Join(sandboxArgs(args), " ")
	if !strings.Contains(strict, "--cgroupns private") || !strings.Contains(strict, "--pids-limit 128") || !strings.Contains(strict, "--cpus 1") {
		t.Fatal(strict)
	}
	t.Setenv("TOOLDECK_SANDBOX_PROFILE", "synology")
	compat := strings.Join(sandboxArgs(args), " ")
	if strings.Contains(compat, "--cgroupns") || strings.Contains(compat, "--pids-limit") || !strings.Contains(compat, "--cpuset-cpus 0") || !strings.Contains(compat, "--cap-drop ALL") {
		t.Fatal(compat)
	}
}
