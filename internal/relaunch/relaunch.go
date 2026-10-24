// Package relaunch replaces the running server with a fresh copy of itself, so
// a restart frees everything the old one held without each part cleaning up.
package relaunch

import (
	"fmt"
	"os"
)

// ExitCode (EX_TEMPFAIL) is used where the process cannot replace itself
// (Windows) or the replacement failed. Non-zero, so systemd's
// Restart=on-failure starts it again; the desktop app's heartbeat does the same.
const ExitCode = 75

// Self execs the current binary with the same arguments and environment.
// It only returns on failure.
func Self() error {
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("relaunch: %w", err)
	}
	return execSelf(bin, os.Args, os.Environ())
}

// SelfOrExit relaunches, or exits with ExitCode when that is not possible.
func SelfOrExit() {
	err := Self()
	fmt.Fprintf(os.Stderr, "%v; exiting for the supervisor to restart\n", err)
	os.Exit(ExitCode)
}
