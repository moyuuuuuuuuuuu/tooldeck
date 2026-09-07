package platform

import "os"

// Compatibility is opt-in for Synology kernels lacking CFS, PID and cgroup namespace support.
func sandboxArgs(args []string) []string {
	limits := []string{"--init", "--ipc", "private", "--shm-size", "16m", "--ulimit", "core=0:0", "--ulimit", "nofile=1024:1024", "--ulimit", "fsize=268435456:268435456", "--log-driver", "none"}
	if os.Getenv("TOOLDECK_SANDBOX_PROFILE") == "synology" {
		filtered := []string{args[0]}
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--pids-limit":
				i++
			case "--cpus":
				i++
				filtered = append(filtered, "--cpuset-cpus", "0")
			default:
				filtered = append(filtered, args[i])
			}
		}
		args = filtered
	} else {
		limits = append(limits, "--cgroupns", "private")
	}
	return append([]string{args[0]}, append(limits, args[1:]...)...)
}
