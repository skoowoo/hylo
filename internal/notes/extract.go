package notes

import (
	"fmt"
	"io"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/util"
)

// ReadSource returns the raw markdown of a note given a path or bare filename.
func ReadSource(c *client.Client, pathOrName string) ([]byte, error) {
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func Section(source []byte, heading string) (string, error) {
	sec := client.ParseSection(source, heading)
	if sec == nil {
		return "", fmt.Errorf("no heading matching %q found", heading)
	}
	return string(sec), nil
}

// Tags returns the front matter tags and whether the note has front matter at all.
func Tags(source []byte) (hasFrontMatter bool, tags []string) {
	fm, _ := util.ParseFrontmatter(source)
	if !fm.HasMeta() {
		return false, []string{}
	}
	if fm.Tags == nil {
		return true, []string{}
	}
	return true, fm.Tags
}

// SegmentSpec selects lines by Head, Tail, or the 1-based inclusive Start/End range; exactly one form.
type SegmentSpec struct {
	Head, Tail, Start, End int
}

func (s SegmentSpec) Validate() error {
	n := 0
	for _, set := range []bool{s.Head > 0, s.Tail > 0, s.Start > 0 || s.End > 0} {
		if set {
			n++
		}
	}
	switch {
	case n == 0:
		return fmt.Errorf("specify one of head, tail, or start/end")
	case n > 1:
		return fmt.Errorf("head, tail, and start/end are mutually exclusive")
	case (s.Start > 0) != (s.End > 0):
		return fmt.Errorf("start and end must be used together")
	case s.Start > s.End:
		return fmt.Errorf("start (%d) must not be greater than end (%d)", s.Start, s.End)
	}
	return nil
}

// Segment returns the selected lines (nil when the range is empty) and the note's total line count.
func Segment(text string, s SegmentSpec) (selected []string, total int) {
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total = len(lines)

	var lo, hi int
	switch {
	case s.Head > 0:
		hi = s.Head
	case s.Tail > 0:
		lo, hi = total-s.Tail, total
	default:
		lo, hi = s.Start-1, s.End
	}
	lo = max(lo, 0)
	hi = min(hi, total)
	if lo >= hi {
		return nil, total
	}
	return lines[lo:hi], total
}
