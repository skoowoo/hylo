//go:build windows

package agent

import "os/exec"

func detachShellProcess(cmd *exec.Cmd) {}
