package relaunch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func children() []int {
	files, _ := filepath.Glob("/proc/self/task/*/children")
	var out []int
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(b)) {
			if pid, err := strconv.Atoi(field); err == nil {
				out = append(out, pid)
			}
		}
	}
	return out
}
