//go:build unix

package relaunch

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// execSelf keeps the PID, so the desktop app's child handle, the pid file and
// systemd all still point at the server.
func execSelf(bin string, args, env []string) error {
	reapChildren(2 * time.Second)
	if err := syscall.Exec(bin, args, env); err != nil {
		return fmt.Errorf("relaunch exec %s: %w", bin, err)
	}
	return nil
}

// reapChildren kills and waits for every child still left, such as an agent
// probe. The new image doesn't know their PIDs, so they would stay zombies.
func reapChildren(within time.Duration) {
	for _, pid := range children() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if errors.Is(err, syscall.ECHILD) {
			return
		}
		if pid > 0 || errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
