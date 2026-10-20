package qrpng

import (
	"strings"
	"testing"
)

func TestDataURL(t *testing.T) {
	url, err := DataURL("https://liteapp.weixin.qq.com/q/test?qrcode=abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(url) < 50 || !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("unexpected data url prefix: %q", url[:min(30, len(url))])
	}
	if _, err := DataURL(" "); err == nil {
		t.Fatal("expected error for empty payload")
	}
}
