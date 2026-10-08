//go:build unix

package agent

import (
	"os/exec"
	"syscall"
)

// Interactive login shells call tcsetpgrp on the controlling terminal.
// That moves the server out of the foreground process group; the next write
// stops it with SIGTTOU, and a stopped process does not run SIGINT/SIGTERM.
func detachShellProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
