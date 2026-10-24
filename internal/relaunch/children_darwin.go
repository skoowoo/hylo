package relaunch

import (
	"os"

	"golang.org/x/sys/unix"
)

func children() []int {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil
	}
	self := int32(os.Getpid())
	var out []int
	for _, p := range procs {
		if p.Eproc.Ppid == self {
			out = append(out, int(p.Proc.P_pid))
		}
	}
	return out
}
