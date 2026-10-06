package notes

import (
	"fmt"

	"github.com/hardhacker/hylo/internal/client"
)

type ShortQuery struct {
	Dir   string // default _shorts
	Limit int
	Time  TimeFilter
}

func ShortCreate(c *client.Client, content, dir string) (path string, err error) {
	if len([]rune(content)) == 0 {
		return "", fmt.Errorf("content must not be empty")
	}
	note, err := c.CreateShort(content, dir)
	if err != nil {
		return "", err
	}
	return note.PathString(), nil
}

func ShortList(c *client.Client, q ShortQuery) ([]client.ShortEntry, error) {
	opts := client.ShortListOptions{Dir: q.Dir, Limit: q.Limit}
	var err error
	if opts.After, opts.Before, err = q.Time.Bounds(); err != nil {
		return nil, err
	}
	entries, err := c.ListShorts(opts)
	if entries == nil {
		entries = []client.ShortEntry{}
	}
	return entries, err
}

func TagList(c *client.Client, limit int) ([]client.TagStat, error) {
	resp, err := c.TagList(limit)
	if err != nil {
		return nil, err
	}
	if resp.Tags == nil {
		return []client.TagStat{}, nil
	}
	return resp.Tags, nil
}
