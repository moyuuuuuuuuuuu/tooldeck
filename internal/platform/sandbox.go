package platform

// Keep PID/network namespaces and Docker default seccomp; never use host or unconfined modes.
func sandboxLimits() []string {
	return []string{"--init", "--ipc", "private", "--cgroupns", "private", "--shm-size", "16m", "--ulimit", "core=0:0", "--ulimit", "nofile=1024:1024", "--ulimit", "fsize=268435456:268435456", "--log-driver", "none"}
}
