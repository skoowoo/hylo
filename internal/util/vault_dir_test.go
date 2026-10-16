package util

import "testing"

func TestCleanVaultDir(t *testing.T) {
	for in, want := range map[string]string{"": "/", "/": "/", "/_news/": "/_news", "/a//b": "/a/b"} {
		if got, err := CleanVaultDir(in); err != nil || got != want {
			t.Errorf("CleanVaultDir(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"_news", "/../etc", "/a/./b", "/.hylo"} {
		if _, err := CleanVaultDir(bad); err == nil {
			t.Errorf("CleanVaultDir(%q) accepted", bad)
		}
	}
}
