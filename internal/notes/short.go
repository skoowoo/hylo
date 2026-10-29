package notes

import (
	"fmt"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
)

type ShortQuery struct {
	Limit int
	Time  TimeFilter
}

func ShortCreate(c *client.Client, content string) (client.ShortEntry, error) {
	if strings.TrimSpace(content) == "" {
		return client.ShortEntry{}, fmt.Errorf("content must not be empty")
	}
	return c.CreateShort(content)
}

func ShortList(c *client.Client, q ShortQuery) ([]client.ShortEntry, error) {
	opts := client.ShortListOptions{Limit: q.Limit}
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

func ShortUpdate(c *client.Client, id, hash, content string) (client.ShortEntry, error) {
	if strings.TrimSpace(content) == "" {
		return client.ShortEntry{}, fmt.Errorf("content must not be empty; delete the short note instead")
	}
	return c.UpdateShort(id, hash, content)
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
