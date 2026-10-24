//go:build darwin || linux

package relaunch

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestReapChildrenLeavesNoZombie(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	reapChildren(2 * time.Second)

	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil); !errors.Is(err, syscall.ECHILD) {
		t.Fatalf("child %d still ours: %v", pid, err)
	}
}
