package search

import (
	"testing"

	"github.com/hardhacker/hylo/internal/util"
)

func TestExtractSearchTags(t *testing.T) {
	raw := []byte(`---
tags:
  - alpha
  - beta-gamma
distilled: 2026-01-01
---

# Body
`)
	fm, _ := util.ParseFrontmatter(raw)
	if len(fm.Tags) != 2 {
		t.Fatalf("parse: %#v", fm.Tags)
	}
	got := extractSearchTags(raw)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta-gamma" {
		t.Fatalf("extractSearchTags: %#v", got)
	}
}

func TestExtractSearchTags_none(t *testing.T) {
	raw := []byte(`# no front matter`)
	if extractSearchTags(raw) != nil {
		t.Fatal("expected nil tags")
	}
}
