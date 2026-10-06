// Package notes holds the vault operations shared by the hylo CLI and the MCP server.
// Callers own argument parsing and output formatting; validation and rules live here.
package notes

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/util"
)

func CheckAbs(field, p string) error {
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("%s %q must be absolute (start with \"/\")", field, p)
	}
	return nil
}

func CheckMarkdownPath(p string) error {
	if err := CheckAbs("path", p); err != nil {
		return err
	}
	if !util.IsMarkdownPath(p) {
		return fmt.Errorf("path %q is not a markdown file (.md or .markdown)", p)
	}
	return nil
}

func CheckText(data []byte) error {
	if !util.IsValidText(data) {
		return fmt.Errorf("content appears to be binary — only text can be stored as markdown")
	}
	return nil
}

func EnsureMarkdownExt(name string) string {
	name = strings.TrimSpace(name)
	if path.Ext(name) != "" {
		return name
	}
	return name + ".md"
}

// TimeFilter narrows results by updated/created time. Latest cannot be combined with Start/End.
type TimeFilter struct {
	Start  string // YYYY-MM-DD, inclusive
	End    string // YYYY-MM-DD, exclusive
	Latest int    // last N days
}

func (f TimeFilter) Bounds() (after, before time.Time, err error) {
	if f.Latest > 0 && (f.Start != "" || f.End != "") {
		return after, before, fmt.Errorf("cannot combine latest with start/end")
	}
	if f.Latest > 0 {
		after = time.Now().AddDate(0, 0, -f.Latest)
	}
	if f.Start != "" {
		if after, err = time.Parse(time.DateOnly, f.Start); err != nil {
			return after, before, fmt.Errorf("invalid start date (use YYYY-MM-DD): %w", err)
		}
	}
	if f.End != "" {
		if before, err = time.Parse(time.DateOnly, f.End); err != nil {
			return after, before, fmt.Errorf("invalid end date (use YYYY-MM-DD): %w", err)
		}
	}
	return after, before, nil
}
