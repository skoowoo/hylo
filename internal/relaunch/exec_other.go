//go:build !unix

package relaunch

import "errors"

func execSelf(string, []string, []string) error {
	return errors.New("relaunch: exec is not supported on this platform")
}
